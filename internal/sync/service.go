package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ai-tutor-local/internal/db"

	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// Service handles local analytics queueing for cloud sync.
type Service struct {
	queries *db.SyncQueueQueries
}

type SyncStatus struct {
	PendingCount  int64  `json:"pending_count"`
	LastSyncTime  string `json:"last_sync_time,omitempty"`
	Health        string `json:"health"`
	NextRetryInMS int64  `json:"next_retry_in_ms"`
}

type ManualSyncResult struct {
	Attempted int `json:"attempted"`
	Sent      int `json:"sent"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

func NewService(database *db.Database) *Service {
	return &Service{
		queries: db.NewSyncQueueQueries(database.DB),
	}
}

func (s *Service) GetStatus() (*SyncStatus, error) {
	pendingCount, err := s.queries.CountPendingAndFailed()
	if err != nil {
		return nil, fmt.Errorf("sync status count: %w", err)
	}

	lastSyncAt, err := s.queries.LastSuccessfulSyncAt()
	if err != nil {
		return nil, fmt.Errorf("sync status last success: %w", err)
	}

	items, err := s.queries.ListRetryable(100)
	if err != nil {
		return nil, fmt.Errorf("sync status retryable list: %w", err)
	}

	now := time.Now().UTC()
	minRetryDelay := int64(0)
	health := "ok"
	lastSync := ""
	if lastSyncAt != nil {
		lastSync = lastSyncAt.UTC().Format(time.RFC3339)
	}
	if pendingCount > 0 {
		health = "backlog"
	}

	for _, item := range items {
		delay := retryDelay(item, now)
		if minRetryDelay == 0 || delay < minRetryDelay {
			minRetryDelay = delay
		}
		if item.Status == "failed" {
			health = "degraded"
		}
	}

	return &SyncStatus{
		PendingCount:  pendingCount,
		LastSyncTime:  lastSync,
		Health:        health,
		NextRetryInMS: minRetryDelay,
	}, nil
}

func (s *Service) RunManualSync(ctx context.Context, limit int) (*ManualSyncResult, error) {
	if limit <= 0 {
		limit = 50
	}

	items, err := s.queries.ListRetryable(limit)
	if err != nil {
		return nil, fmt.Errorf("list retryable queue items: %w", err)
	}

	result := &ManualSyncResult{}
	now := time.Now().UTC()
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("manual sync canceled: %w", err)
		}

		delay := retryDelay(item, now)
		if delay > 0 {
			result.Skipped++
			continue
		}

		result.Attempted++

		var event Event
		if err := json.Unmarshal([]byte(item.Payload), &event); err != nil {
			if markErr := s.queries.MarkAttempt(item.ID, "failed"); markErr != nil {
				return nil, fmt.Errorf("mark invalid payload failed for %s: %w", item.ID, markErr)
			}
			result.Failed++
			continue
		}

		if err := validateEvent(event); err != nil {
			if markErr := s.queries.MarkAttempt(item.ID, "failed"); markErr != nil {
				return nil, fmt.Errorf("mark invalid event failed for %s: %w", item.ID, markErr)
			}
			result.Failed++
			continue
		}

		if err := s.queries.MarkAttempt(item.ID, "sent"); err != nil {
			return nil, fmt.Errorf("mark sync item sent for %s: %w", item.ID, err)
		}
		result.Sent++
	}

	return result, nil
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

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code == sqlite3.ErrConstraint {
			return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
		}
	}

	return false
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
	if requiresActivityType(event.EventType) && strings.TrimSpace(event.ActivityType) == "" {
		return fmt.Errorf("enqueue event: activity_type is required for event_type=%s", event.EventType)
	}
	if requiresActivityType(event.EventType) && strings.TrimSpace(event.ActivityType) != "" {
		if !isValidActivityType(event.ActivityType) {
			return fmt.Errorf("enqueue event: invalid activity_type=%s (must be one of: flashcard, quiz, reading, search)", event.ActivityType)
		}
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

func requiresActivityType(eventType string) bool {
	switch strings.TrimSpace(strings.ToLower(eventType)) {
	case "quiz_completed", "flashcard_session", "flashcard_session_completed", "study_session":
		return true
	default:
		return false
	}
}

func isValidActivityType(activityType string) bool {
	normalized := strings.TrimSpace(strings.ToLower(activityType))
	switch normalized {
	case "flashcard", "quiz", "reading", "search":
		return true
	default:
		return false
	}
}

func retryDelay(item db.SyncQueueItem, now time.Time) int64 {
	if item.LastAttempt == nil {
		return 0
	}

	backoff := backoffDuration(item.Attempts)
	nextAttemptAt := item.LastAttempt.UTC().Add(backoff)
	if !nextAttemptAt.After(now) {
		return 0
	}

	return nextAttemptAt.Sub(now).Milliseconds()
}

func backoffDuration(attempts int) time.Duration {
	if attempts <= 0 {
		return 0
	}

	minutes := 1 << (attempts - 1)
	if minutes > 32 {
		minutes = 32
	}
	return time.Duration(minutes) * time.Minute
}
