package generation

import "context"

// Client is provider-agnostic text generation interface.
// Embeddings are intentionally not part of this interface.
type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
}
