package sync

import (
	"encoding/json"
	"fmt"
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
	return s.queries.Enqueue(item)
}
