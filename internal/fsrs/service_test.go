package fsrs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ai-tutor-local/internal/db"
	syncsvc "ai-tutor-local/internal/sync"

	"github.com/google/uuid"
)

func TestReviewCardAndCompleteSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	database, err := db.Init(filepath.Join(dir, "fsrs_test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	now := time.Now().UTC()
	notebook := &db.Notebook{
		ID:          uuid.NewString(),
		Name:        "FSRS Test",
		Description: "",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := database.DB.Create(notebook).Error; err != nil {
		t.Fatalf("create notebook: %v", err)
	}

	document := &db.Document{
		ID:         uuid.NewString(),
		NotebookID: notebook.ID,
		Filename:   "seed.txt",
		FilePath:   "seed.txt",
		FileHash:   uuid.NewString(),
		Status:     "ready",
		CreatedAt:  now,
	}
	if err := database.DB.Create(document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}

	chunk := &db.Chunk{
		ID:            uuid.NewString(),
		DocumentID:    document.ID,
		NotebookID:    notebook.ID,
		ChapterName:   "General",
		ChunkIndex:    0,
		Content:       "seed content",
		TaggedContent: "[FSRS Test - General] seed content",
		TokenCount:    3,
		CreatedAt:     now,
	}
	if err := database.DB.Create(chunk).Error; err != nil {
		t.Fatalf("create chunk: %v", err)
	}

	card := &db.Flashcard{
		ID:             uuid.NewString(),
		ChunkID:        chunk.ID,
		NotebookID:     notebook.ID,
		Question:       "What is FSRS?",
		Answer:         "A spaced repetition scheduler",
		Source:         "ai",
		Stability:      0.4,
		Difficulty:     5,
		Retrievability: 1,
		State:          "new",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := database.DB.Create(card).Error; err != nil {
		t.Fatalf("create flashcard: %v", err)
	}

	svc := NewService(database, syncsvc.NewService(database))

	res, err := svc.ReviewCard(ctx, ReviewInput{
		FlashcardID: card.ID,
		NotebookID:  notebook.ID,
		Rating:      RatingGood,
		TimeTakenMs: 1800,
	})
	if err != nil {
		t.Fatalf("review card: %v", err)
	}

	if res.Reps != 1 {
		t.Fatalf("expected reps=1, got %d", res.Reps)
	}
	if res.NextDueAt.Before(now.Add(2 * 24 * time.Hour)) {
		t.Fatalf("expected due date to move forward, got %s", res.NextDueAt)
	}

	sessionStart := time.Now().UTC().Add(-5 * time.Minute)
	sessionEnd := time.Now().UTC()
	if err := svc.CompleteSession(ctx, SessionSummary{
		NotebookID:         notebook.ID,
		NotebookName:       notebook.Name,
		StartedAt:          sessionStart,
		EndedAt:            sessionEnd,
		FlashcardsReviewed: 1,
		CorrectRecallCount: 1,
		TotalTimeTakenMS:   1800,
		EmitTelemetry:      true,
	}); err != nil {
		t.Fatalf("complete session: %v", err)
	}

	var reviewLogCount int64
	if err := database.DB.Raw("SELECT COUNT(1) FROM review_logs").Scan(&reviewLogCount).Error; err != nil {
		t.Fatalf("count review logs: %v", err)
	}
	if reviewLogCount != 1 {
		t.Fatalf("expected 1 review log, got %d", reviewLogCount)
	}

	var syncQueueCount int64
	if err := database.DB.Raw("SELECT COUNT(1) FROM sync_queue").Scan(&syncQueueCount).Error; err != nil {
		t.Fatalf("count sync queue: %v", err)
	}
	if syncQueueCount != 1 {
		t.Fatalf("expected 1 sync event, got %d", syncQueueCount)
	}
}
