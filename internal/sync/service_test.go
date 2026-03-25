package sync

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"ai-tutor-local/internal/db"

	"github.com/google/uuid"
)

func TestEnqueueDuplicateEventIDIsIdempotent(t *testing.T) {
	database := testDB(t)
	svc := NewService(database)

	event := Event{
		EventID:          uuid.NewString(),
		EventType:        "flashcard_session_completed",
		ActivityType:     "flashcard",
		NotebookID:       uuid.NewString(),
		NotebookName:     "Sprint4 Notebook",
		TimeSpentSeconds: 120,
		OccurredAt:       time.Now().UTC(),
	}

	if err := svc.Enqueue(event); err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	if err := svc.Enqueue(event); err != nil {
		t.Fatalf("duplicate enqueue should be idempotent, got error: %v", err)
	}

	var count int64
	if err := database.DB.Table("sync_queue").Where("id = ?", event.EventID).Count(&count).Error; err != nil {
		t.Fatalf("count queued event rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 queued row for duplicate event id, got %d", count)
	}
}

func TestEnqueueRejectsInvalidMetrics(t *testing.T) {
	database := testDB(t)
	svc := NewService(database)

	badAccuracy := float32(101)
	err := svc.Enqueue(Event{
		EventID:          uuid.NewString(),
		EventType:        "flashcard_session_completed",
		ActivityType:     "flashcard",
		NotebookID:       uuid.NewString(),
		NotebookName:     "Sprint4 Notebook",
		TimeSpentSeconds: 50,
		AccuracyPct:      &badAccuracy,
		OccurredAt:       time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected validation error for out-of-range accuracy")
	}
}

func TestRunManualSyncMarksReadyItemsSentAndRespectsBackoff(t *testing.T) {
	database := testDB(t)
	svc := NewService(database)

	now := time.Now().UTC()

	ready := Event{
		EventID:          uuid.NewString(),
		EventType:        "flashcard_session_completed",
		ActivityType:     "flashcard",
		NotebookID:       uuid.NewString(),
		NotebookName:     "Sprint5 Notebook",
		TimeSpentSeconds: 90,
		OccurredAt:       now,
	}
	readyPayload, err := json.Marshal(ready)
	if err != nil {
		t.Fatalf("marshal ready payload: %v", err)
	}
	if err := database.DB.Create(&db.SyncQueueItem{
		ID:        ready.EventID,
		Payload:   string(readyPayload),
		CreatedAt: now,
		Status:    "pending",
	}).Error; err != nil {
		t.Fatalf("insert ready item: %v", err)
	}

	deferred := Event{
		EventID:          uuid.NewString(),
		EventType:        "flashcard_session_completed",
		ActivityType:     "flashcard",
		NotebookID:       uuid.NewString(),
		NotebookName:     "Sprint5 Notebook",
		TimeSpentSeconds: 120,
		OccurredAt:       now,
	}
	deferredPayload, err := json.Marshal(deferred)
	if err != nil {
		t.Fatalf("marshal deferred payload: %v", err)
	}
	lastAttempt := now.Add(-500 * time.Millisecond)
	if err := database.DB.Create(&db.SyncQueueItem{
		ID:          deferred.EventID,
		Payload:     string(deferredPayload),
		CreatedAt:   now,
		Attempts:    1,
		LastAttempt: &lastAttempt,
		Status:      "failed",
	}).Error; err != nil {
		t.Fatalf("insert deferred item: %v", err)
	}

	result, err := svc.RunManualSync(context.Background(), 10)
	if err != nil {
		t.Fatalf("run manual sync: %v", err)
	}

	if result.Sent != 1 {
		t.Fatalf("expected 1 sent item, got %d", result.Sent)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected 1 skipped item due to backoff, got %d", result.Skipped)
	}

	var readyRow db.SyncQueueItem
	if err := database.DB.First(&readyRow, "id = ?", ready.EventID).Error; err != nil {
		t.Fatalf("load ready row: %v", err)
	}
	if readyRow.Status != "sent" {
		t.Fatalf("expected ready item status sent, got %s", readyRow.Status)
	}

	var deferredRow db.SyncQueueItem
	if err := database.DB.First(&deferredRow, "id = ?", deferred.EventID).Error; err != nil {
		t.Fatalf("load deferred row: %v", err)
	}
	if deferredRow.Status != "failed" {
		t.Fatalf("expected deferred item to stay failed while backoff active, got %s", deferredRow.Status)
	}
}

func TestGetStatusReturnsBacklogAndRetryWindow(t *testing.T) {
	database := testDB(t)
	svc := NewService(database)

	event := Event{
		EventID:          uuid.NewString(),
		EventType:        "flashcard_session_completed",
		ActivityType:     "flashcard",
		NotebookID:       uuid.NewString(),
		NotebookName:     "Sprint5 Notebook",
		TimeSpentSeconds: 42,
		OccurredAt:       time.Now().UTC(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	lastAttempt := time.Now().UTC()
	if err := database.DB.Create(&db.SyncQueueItem{
		ID:          event.EventID,
		Payload:     string(payload),
		CreatedAt:   time.Now().UTC(),
		Attempts:    2,
		LastAttempt: &lastAttempt,
		Status:      "failed",
	}).Error; err != nil {
		t.Fatalf("insert sync item: %v", err)
	}

	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("get status: %v", err)
	}

	if status.PendingCount != 1 {
		t.Fatalf("expected pending count 1, got %d", status.PendingCount)
	}
	if status.Health != "degraded" {
		t.Fatalf("expected degraded health, got %s", status.Health)
	}
	if status.NextRetryInMS <= 0 {
		t.Fatalf("expected positive next retry delay, got %d", status.NextRetryInMS)
	}
}

func testDB(t *testing.T) *db.Database {
	t.Helper()

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "sync-test.db")

	database, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := database.DB.AutoMigrate(&db.SyncQueueItem{}); err != nil {
		t.Fatalf("automigrate sync queue: %v", err)
	}

	return database
}
