package embedding

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestONNXEmbeddingOutputShape768(t *testing.T) {
	modelPath := resolveModelPath(t)
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("onnx model unavailable at %s: %v", modelPath, err)
	}

	client, err := NewONNXClient(modelPath)
	if err != nil {
		t.Skipf("onnx runtime unavailable for integration test: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vectors, err := client.EmbedText(ctx, []string{
		"Fundamental rights are enforceable through constitutional remedies.",
		"Directive principles guide governance and social justice goals.",
	})
	if err != nil {
		t.Fatalf("embed text with onnx model: %v", err)
	}

	if got, want := len(vectors), 2; got != want {
		t.Fatalf("unexpected embedding count: got %d want %d", got, want)
	}

	for i, row := range vectors {
		if got, want := len(row), 768; got != want {
			t.Fatalf("embedding[%d] shape mismatch: got %d want %d", i, got, want)
		}
	}
}

func resolveModelPath(t *testing.T) string {
	t.Helper()

	candidates := []string{
		filepath.Clean(filepath.Join("..", "..", "onnx", "model_int8.onnx")),
		filepath.Clean(filepath.Join("onnx", "model_int8.onnx")),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return candidates[0]
}
