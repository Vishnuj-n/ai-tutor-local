package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client provides embedding operations against local Ollama.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// EmbedRequest is the Ollama embed API payload.
type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbedResponse captures Ollama embed response.
type EmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// EmbedText embeds one or more texts using local nomic-embed-text (768 dims).
func (c *Client) EmbedText(ctx context.Context, texts []string) ([][]float32, error) {
	payload := EmbedRequest{
		Model: "nomic-embed-text",
		Input: texts,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama embed api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed api returned status: %d", resp.StatusCode)
	}

	var out EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	return out.Embeddings, nil
}
