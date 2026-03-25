package generation

import "context"

// QuizQuestion is a provider-agnostic quiz question shape.
type QuizQuestion struct {
	Prompt       string   `json:"prompt"`
	Choices      []string `json:"choices,omitempty"`
	Answer       string   `json:"answer,omitempty"`
	Explanation  string   `json:"explanation,omitempty"`
	QuestionType string   `json:"question_type,omitempty"`
}

// QuizResult is a provider-agnostic quiz generation payload.
type QuizResult struct {
	Title     string         `json:"title,omitempty"`
	Questions []QuizQuestion `json:"questions"`
}

// LLMProvider defines the cloud/local generation operations used by the app.
// Embeddings are intentionally not part of this interface and remain local.
type LLMProvider interface {
	GenerateAnswer(ctx context.Context, prompt string) (string, error)
	GenerateQuiz(ctx context.Context, chunks []string) (*QuizResult, error)
	GenerateStream(ctx context.Context, prompt string, onToken func(token string) error) error
}

// Client is kept as a compatibility alias while the codebase transitions.
type Client = LLMProvider
