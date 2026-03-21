package embedding

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	defaultSeqLen    = 128
	targetEmbeddingD = 768
	clsTokenID       = int64(101)
	sepTokenID       = int64(102)
	padTokenID       = int64(0)
	vocabModulo      = int64(30000)
	vocabOffset      = int64(1000)
)

var (
	onceInitONNX sync.Once
	onceInitErr  error
)

// ONNXClient embeds text using a local ONNX model file.
type ONNXClient struct {
	modelPath string
	seqLen    int
	session   *ort.DynamicAdvancedSession
	inputs    []ort.InputOutputInfo
	outputs   []ort.InputOutputInfo
}

// NewONNXClient creates a new ONNX embedding client for model_int8.onnx style models.
func NewONNXClient(modelPath string) (*ONNXClient, error) {
	absModel, err := filepath.Abs(modelPath)
	if err != nil {
		return nil, fmt.Errorf("resolve model path: %w", err)
	}
	if _, err := os.Stat(absModel); err != nil {
		return nil, fmt.Errorf("onnx model not found at %s: %w", absModel, err)
	}

	if err := initONNXRuntime(); err != nil {
		return nil, err
	}

	inputs, outputs, err := ort.GetInputOutputInfo(absModel)
	if err != nil {
		return nil, fmt.Errorf("inspect onnx io: %w", err)
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		return nil, fmt.Errorf("onnx model has empty io metadata")
	}

	inputNames := make([]string, 0, len(inputs))
	for _, in := range inputs {
		inputNames = append(inputNames, in.Name)
	}
	outNames := []string{outputs[0].Name}

	session, err := ort.NewDynamicAdvancedSession(absModel, inputNames, outNames, nil)
	if err != nil {
		return nil, fmt.Errorf("create onnx session: %w", err)
	}

	return &ONNXClient{
		modelPath: absModel,
		seqLen:    defaultSeqLen,
		session:   session,
		inputs:    inputs,
		outputs:   outputs,
	}, nil
}

// Close releases ONNX session resources.
func (c *ONNXClient) Close() error {
	if c == nil || c.session == nil {
		return nil
	}
	err := c.session.Destroy()
	c.session = nil
	return err
}

// EmbedText returns one 768-dimensional embedding per input text.
func (c *ONNXClient) EmbedText(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if c.session == nil {
		return nil, fmt.Errorf("onnx session is not initialized")
	}

	inputIDs, attentionMask, tokenTypeIDs := c.encodeBatch(texts)
	shape := ort.NewShape(int64(len(texts)), int64(c.seqLen))

	inputValues := make([]ort.Value, 0, len(c.inputs))
	for _, info := range c.inputs {
		nameLower := strings.ToLower(info.Name)
		switch {
		case strings.Contains(nameLower, "input") && strings.Contains(nameLower, "id"):
			v, err := newIntTensorForType(info.DataType, shape, inputIDs)
			if err != nil {
				return nil, fmt.Errorf("create input_ids tensor: %w", err)
			}
			inputValues = append(inputValues, v)
		case strings.Contains(nameLower, "attention") && strings.Contains(nameLower, "mask"):
			v, err := newIntTensorForType(info.DataType, shape, attentionMask)
			if err != nil {
				return nil, fmt.Errorf("create attention_mask tensor: %w", err)
			}
			inputValues = append(inputValues, v)
		case strings.Contains(nameLower, "token") && strings.Contains(nameLower, "type"):
			v, err := newIntTensorForType(info.DataType, shape, tokenTypeIDs)
			if err != nil {
				return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
			}
			inputValues = append(inputValues, v)
		default:
			v, err := newIntTensorForType(info.DataType, shape, inputIDs)
			if err != nil {
				return nil, fmt.Errorf("create fallback input tensor %q: %w", info.Name, err)
			}
			inputValues = append(inputValues, v)
		}
	}
	defer destroyValues(inputValues)

	outShape := c.resolveOutputShape(int64(len(texts)), int64(c.seqLen))
	outputTensor, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return nil, fmt.Errorf("allocate output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	outputs := []ort.Value{outputTensor}
	if err := c.session.Run(inputValues, outputs); err != nil {
		return nil, fmt.Errorf("run onnx embedding inference: %w", err)
	}

	return projectEmbeddings(outputs[0], int64(len(texts)), int64(c.seqLen))
}

func initONNXRuntime() error {
	onceInitONNX.Do(func() {
		if p := strings.TrimSpace(os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH")); p != "" {
			ort.SetSharedLibraryPath(p)
		} else if runtime.GOOS == "windows" {
			candidate := filepath.Join("onnx", "onnxruntime.dll")
			if _, err := os.Stat(candidate); err == nil {
				ort.SetSharedLibraryPath(candidate)
			}
		}

		onceInitErr = ort.InitializeEnvironment()
		if onceInitErr != nil {
			onceInitErr = fmt.Errorf("initialize onnx runtime environment: %w", onceInitErr)
		}
	})
	return onceInitErr
}

func newIntTensorForType(dataType ort.TensorElementDataType, shape ort.Shape, data []int64) (ort.Value, error) {
	switch dataType {
	case ort.TensorElementDataTypeInt64:
		return ort.NewTensor(shape, data)
	case ort.TensorElementDataTypeInt32:
		converted := make([]int32, len(data))
		for i := range data {
			converted[i] = int32(data[i])
		}
		return ort.NewTensor(shape, converted)
	case ort.TensorElementDataTypeInt16:
		converted := make([]int16, len(data))
		for i := range data {
			converted[i] = int16(data[i])
		}
		return ort.NewTensor(shape, converted)
	case ort.TensorElementDataTypeInt8:
		converted := make([]int8, len(data))
		for i := range data {
			converted[i] = int8(data[i])
		}
		return ort.NewTensor(shape, converted)
	default:
		return nil, fmt.Errorf("unsupported integer tensor datatype: %s", dataType.String())
	}
}

func destroyValues(values []ort.Value) {
	for _, v := range values {
		if v != nil {
			_ = v.Destroy()
		}
	}
}

func (c *ONNXClient) resolveOutputShape(batch, seqLen int64) ort.Shape {
	if len(c.outputs) == 0 {
		return ort.NewShape(batch, targetEmbeddingD)
	}

	dims := c.outputs[0].Dimensions
	if len(dims) == 0 {
		return ort.NewShape(batch, targetEmbeddingD)
	}

	resolved := make([]int64, len(dims))
	for i := range dims {
		if dims[i] > 0 {
			resolved[i] = dims[i]
			continue
		}
		switch i {
		case 0:
			resolved[i] = batch
		case 1:
			resolved[i] = seqLen
		default:
			resolved[i] = targetEmbeddingD
		}
	}

	return ort.Shape(resolved)
}

func projectEmbeddings(v ort.Value, batch, seqLen int64) ([][]float32, error) {
	t, ok := v.(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("onnx output is not float32 tensor")
	}

	shape := t.GetShape()
	data := t.GetData()

	if len(shape) == 2 {
		if shape[0] != batch {
			return nil, fmt.Errorf("unexpected batch in output: got %d want %d", shape[0], batch)
		}
		dim := shape[1]
		if dim != targetEmbeddingD {
			return nil, fmt.Errorf("unexpected embedding dimension: got %d want %d", dim, targetEmbeddingD)
		}

		out := make([][]float32, batch)
		for b := int64(0); b < batch; b++ {
			start := b * dim
			row := make([]float32, dim)
			copy(row, data[start:start+dim])
			out[b] = row
		}
		return out, nil
	}

	if len(shape) == 3 {
		if shape[0] != batch {
			return nil, fmt.Errorf("unexpected batch in 3D output: got %d want %d", shape[0], batch)
		}
		hidden := shape[2]
		if hidden != targetEmbeddingD {
			return nil, fmt.Errorf("unexpected hidden dimension: got %d want %d", hidden, targetEmbeddingD)
		}

		out := make([][]float32, batch)
		stride := shape[1] * hidden
		if shape[1] <= 0 {
			stride = seqLen * hidden
		}
		for b := int64(0); b < batch; b++ {
			start := b * stride
			emb := make([]float32, hidden)
			for j := int64(0); j < hidden; j++ {
				emb[j] = data[start+j]
			}
			out[b] = emb
		}
		return out, nil
	}

	return nil, fmt.Errorf("unsupported output rank: %d", len(shape))
}

func (c *ONNXClient) encodeBatch(texts []string) ([]int64, []int64, []int64) {
	batch := len(texts)
	total := batch * c.seqLen
	inputIDs := make([]int64, total)
	attentionMask := make([]int64, total)
	tokenTypeIDs := make([]int64, total)

	for i, text := range texts {
		tokens := tokenizeBasic(text)
		start := i * c.seqLen

		inputIDs[start] = clsTokenID
		attentionMask[start] = 1
		pos := 1

		for _, tok := range tokens {
			if pos >= c.seqLen-1 {
				break
			}
			inputIDs[start+pos] = hashTokenID(tok)
			attentionMask[start+pos] = 1
			pos++
		}

		if pos < c.seqLen {
			inputIDs[start+pos] = sepTokenID
			attentionMask[start+pos] = 1
			pos++
		}

		for ; pos < c.seqLen; pos++ {
			inputIDs[start+pos] = padTokenID
			attentionMask[start+pos] = 0
			tokenTypeIDs[start+pos] = 0
		}
	}

	return inputIDs, attentionMask, tokenTypeIDs
}

func tokenizeBasic(text string) []string {
	clean := strings.ToLower(strings.TrimSpace(text))
	if clean == "" {
		return nil
	}
	replacer := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		"\t", " ",
		",", " ",
		".", " ",
		";", " ",
		":", " ",
		"!", " ",
		"?", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"\"", " ",
		"'", " ",
	)
	clean = replacer.Replace(clean)
	parts := strings.Fields(clean)
	return parts
}

func hashTokenID(token string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	return int64(h.Sum32()%uint32(vocabModulo)) + vocabOffset
}
