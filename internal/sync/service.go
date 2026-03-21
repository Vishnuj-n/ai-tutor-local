package sync

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-tutor-local/internal/db"
)

// Service handles local analytics queueing for cloud sync.
type Service struct {
	queries *db.SyncQueueQueries
}

func NewService(database *db.Database) *Service {
	return &Service{
		queries: db.NewSyncQueueQueries(database.DB),
	}
}

// Enqueue stores an event in sync_queue for periodic delivery.
func (s *Service) Enqueue(event Event) error {
	if err := validateEvent(event); err != nil {
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	item := &db.SyncQueueItem{
		ID:        event.EventID,
		Payload:   string(payload),
		CreatedAt: time.Now().UTC(),
		Status:    "pending",
	}
	if err := s.queries.Enqueue(item); err != nil {
		if isDuplicateEventIDError(err) {
			return nil
		}
		return err
	}
	return nil
}

func isDuplicateEventIDError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique constraint") || strings.Contains(lower, "duplicate key")
}

func validateEvent(event Event) error {
	if strings.TrimSpace(event.EventID) == "" {
		return fmt.Errorf("enqueue event: event_id is required")
	}
	if strings.TrimSpace(event.EventType) == "" {
		return fmt.Errorf("enqueue event: event_type is required")
	}
	if strings.TrimSpace(event.NotebookID) == "" {
		return fmt.Errorf("enqueue event: notebook_id is required")
	}
	if event.TimeSpentSeconds < 0 {
		return fmt.Errorf("enqueue event: time_spent_seconds cannot be negative")
	}
	if event.FlashcardsCompleted < 0 {
		return fmt.Errorf("enqueue event: flashcards_completed cannot be negative")
	}
	if event.AccuracyPct != nil {
		if *event.AccuracyPct < 0 || *event.AccuracyPct > 100 {
			return fmt.Errorf("enqueue event: accuracy_pct must be in [0,100]")
		}
	}
	if event.OccurredAt.IsZero() {
		return fmt.Errorf("enqueue event: occurred_at is required")
	}

	return nil
}
