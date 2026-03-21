package sync

import "time"

// Event is the analytics event synced to cloud.
type Event struct {
	EventID             string    `json:"event_id"`
	EventType           string    `json:"event_type"`
	NotebookID          string    `json:"notebook_id"`
	NotebookName        string    `json:"notebook_name"`
	TopicName           string    `json:"topic_name,omitempty"`
	ActivityType        string    `json:"activity_type,omitempty"`
	TimeSpentSeconds    int       `json:"time_spent_seconds,omitempty"`
	FlashcardsCompleted int       `json:"flashcards_completed,omitempty"`
	QuizScore           *int      `json:"quiz_score,omitempty"`
	QuizTotal           *int      `json:"quiz_total,omitempty"`
	AccuracyPct         *float32  `json:"accuracy_pct,omitempty"`
	CurrentStreak       int       `json:"current_streak,omitempty"`
	OccurredAt          time.Time `json:"occurred_at"`
}
