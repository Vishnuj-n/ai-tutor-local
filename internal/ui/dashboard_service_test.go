package ui

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ai-tutor-local/internal/db"

	"github.com/google/uuid"
)

func TestGetSnapshot(t *testing.T) {
	database := testDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	notebook := db.Notebook{
		ID:          uuid.NewString(),
		Name:        "UI Notebook",
		Description: "Dashboard snapshot test",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := database.DB.Create(&notebook).Error; err != nil {
		t.Fatalf("create notebook: %v", err)
	}

	doc := db.Document{
		ID:         uuid.NewString(),
		NotebookID: notebook.ID,
		Filename:   "sample.pdf",
		FilePath:   "sample.pdf",
		FileHash:   uuid.NewString(),
		Status:     "processing",
		CreatedAt:  now,
	}
	if err := database.DB.Create(&doc).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}

	chunk := db.Chunk{
		ID:            uuid.NewString(),
		DocumentID:    doc.ID,
		NotebookID:    notebook.ID,
		ChapterName:   "General",
		ChunkIndex:    0,
		Content:       "Snapshot chunk content",
		TaggedContent: "[UI Notebook - General] Snapshot chunk content",
		TokenCount:    8,
		CreatedAt:     now,
	}
	if err := database.DB.Create(&chunk).Error; err != nil {
		t.Fatalf("create chunk: %v", err)
	}

	flashcard := db.Flashcard{
		ID:         uuid.NewString(),
		ChunkID:    chunk.ID,
		NotebookID: notebook.ID,
		Question:   "Q?",
		Answer:     "A",
		DueDate:    nil,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := database.DB.Create(&flashcard).Error; err != nil {
		t.Fatalf("create flashcard: %v", err)
	}

	study := db.StudySession{
		ID:               uuid.NewString(),
		NotebookID:       notebook.ID,
		ActivityType:     "flashcard",
		TimeSpentSeconds: 120,
		StartedAt:        now.Add(-30 * time.Minute),
		EndedAt:          now,
	}
	if err := database.DB.Create(&study).Error; err != nil {
		t.Fatalf("create study session: %v", err)
	}

	queueItem := db.SyncQueueItem{
		ID:        uuid.NewString(),
		Payload:   `{"event":"x"}`,
		CreatedAt: now,
		Status:    "pending",
	}
	if err := database.DB.Create(&queueItem).Error; err != nil {
		t.Fatalf("create sync queue item: %v", err)
	}

	svc := NewDashboardService(database)
	snapshot, err := svc.GetSnapshot(ctx)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}

	if snapshot.DueToday < 1 {
		t.Fatalf("expected at least 1 due card, got %d", snapshot.DueToday)
	}
	if snapshot.ActiveNotebooks < 1 {
		t.Fatalf("expected at least 1 active notebook, got %d", snapshot.ActiveNotebooks)
	}
	if snapshot.PendingSync < 1 {
		t.Fatalf("expected at least 1 pending sync event, got %d", snapshot.PendingSync)
	}
	if len(snapshot.Ingestion) == 0 {
		t.Fatal("expected ingestion rows in snapshot")
	}
	if snapshot.Ingestion[0].ProgressPct != 55 {
		t.Fatalf("expected processing status progress 55, got %d", snapshot.Ingestion[0].ProgressPct)
	}
}

func testDB(t *testing.T) *db.Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ui-test.db")
	database, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := database.DB.AutoMigrate(
		&db.Notebook{},
		&db.Document{},
		&db.Flashcard{},
		&db.StudySession{},
		&db.SyncQueueItem{},
	); err != nil {
		t.Fatalf("automigrate ui snapshot tables: %v", err)
	}

	return database
}
