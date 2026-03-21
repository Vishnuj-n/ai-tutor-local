package sync

import (
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
