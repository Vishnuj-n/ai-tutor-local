package generation

import "strings"

// TokenBudget defines a hard token ceiling for API requests.
type TokenBudget struct {
	MaxInputTokens int
}

// EstimateTokens performs a conservative token estimate using words.
// This avoids provider lock-in and is sufficient for guardrail truncation.
func EstimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	words := len(strings.Fields(trimmed))
	if words == 0 {
		return 0
	}
	// Roughly 1.3 tokens per English word; rounded up.
	return (words*13 + 9) / 10
}

// PackChunksWithinBudget returns chunks from highest-ranked order that fit the budget.
func PackChunksWithinBudget(chunks []string, budget TokenBudget) []string {
	if budget.MaxInputTokens <= 0 || len(chunks) == 0 {
		return nil
	}

	picked := make([]string, 0, len(chunks))
	used := 0
	for _, chunk := range chunks {
		t := EstimateTokens(chunk)
		if t <= 0 {
			continue
		}
		if used+t > budget.MaxInputTokens {
			break
		}
		picked = append(picked, chunk)
		used += t
	}

	return picked
}
