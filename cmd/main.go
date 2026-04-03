package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-tutor-local/internal/db"
	"ai-tutor-local/internal/embedding"
	"ai-tutor-local/internal/fsrs"
	"ai-tutor-local/internal/ingestion"
	"ai-tutor-local/internal/retrieval"
	"ai-tutor-local/internal/scheduler"
	syncsvc "ai-tutor-local/internal/sync"
	"ai-tutor-local/internal/ui"

	"github.com/google/uuid"
)

func main() {
	ingestFile := flag.String("ingest-file", "", "Absolute or relative file path to ingest (supports text and PDF)")
	notebookName := flag.String("notebook", "Sprint2 Notebook", "Notebook name for ingestion smoke run")
	query := flag.String("query", "", "Optional keyword query to run after ingestion")
	strictVec := flag.Bool("strict-vec", false, "Fail startup if sqlite-vec (vec0) is unavailable")
	reviewSmoke := flag.Bool("review-smoke", false, "Run Sprint 3 FSRS review and telemetry smoke workflow")
	reviewCards := flag.Int("review-cards", 3, "Number of sample flashcards to create for review smoke run")
	dashboardSnapshot := flag.Bool("dashboard-snapshot", false, "Print home dashboard snapshot JSON for frontend wiring")
	flag.Parse()

	dbPath := filepath.Join("data", "app.db")
	schemaPath := "schema.sql"

	database, err := db.Init(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			log.Printf("database close error: %v", closeErr)
		}
	}()

	capabilities, err := database.DetectSQLiteCapabilities()
	if err != nil {
		log.Fatalf("failed to probe SQLite capabilities: %v", err)
	}

	if !capabilities.FTS5 {
		log.Fatal("SQLite FTS5 module is unavailable. Rebuild/run with sqlite FTS5 enabled (example: go run -tags \"sqlite_fts5\" ./cmd)")
	}

	skipVectorTable := false
	if !capabilities.Vec0 {
		if *strictVec || envBool("AI_TUTOR_STRICT_VEC0") {
			log.Fatal("sqlite-vec (vec0) is unavailable and strict vec mode is enabled")
		}

		onnxPath := filepath.Join("onnx", "model_int8.onnx")
		if _, statErr := os.Stat(onnxPath); statErr != nil {
			log.Fatalf("sqlite-vec (vec0) is unavailable and ONNX fallback model is missing at %s: %v", onnxPath, statErr)
		}
		log.Printf("sqlite-vec (vec0) is unavailable; proceeding with ONNX embedding fallback and skipping embeddings virtual table migration")
		skipVectorTable = true
	}

	if err := database.RunSchemaMigrationsWithOptions(schemaPath, db.MigrationOptions{SkipVectorTable: skipVectorTable}); err != nil {
		log.Fatalf("failed to run schema migrations: %v", err)
	}

	if strings.TrimSpace(*ingestFile) != "" {
		if err := runIngestionSmoke(database, capabilities.Vec0, *ingestFile, *notebookName, *query); err != nil {
			log.Fatalf("ingestion smoke run failed: %v", err)
		}
		return
	}

	if *reviewSmoke {
		if err := runReviewSmoke(database, *notebookName, *reviewCards); err != nil {
			log.Fatalf("review smoke run failed: %v", err)
		}
		return
	}

	if *dashboardSnapshot {
		if err := runDashboardSnapshot(database); err != nil {
			log.Fatalf("dashboard snapshot run failed: %v", err)
		}
		return
	}

	fmt.Println("ai-tutor-local Sprint 3 backend baseline ready")
}

func runDashboardSnapshot(database *db.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	svc := ui.NewDashboardService(database)
	snapshot, err := svc.GetSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("load dashboard snapshot: %w", err)
	}

	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dashboard snapshot: %w", err)
	}

	fmt.Println(string(b))
	return nil
}

func runIngestionSmoke(database *db.Database, vecAvailable bool, filePath, notebookName, query string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	nb := &db.Notebook{
		ID:          uuid.NewString(),
		Name:        notebookName,
		Description: "Sprint 2 ingestion smoke run",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := database.DB.WithContext(ctx).Create(nb).Error; err != nil {
		return fmt.Errorf("create notebook: %w", err)
	}

	ingestionSvc := ingestion.NewService(database)
	ingestionSvc.SetVectorStoreEnabled(vecAvailable)
	if vecAvailable {
		onnxPath := filepath.Join("onnx", "model_int8.onnx")
		onnxEmbedder, err := embedding.NewONNXClient(onnxPath)
		if err != nil {
			return fmt.Errorf("initialize onnx embedder: %w", err)
		}
		defer func() {
			_ = onnxEmbedder.Close()
		}()
		ingestionSvc.SetEmbedder(onnxEmbedder)
	}

	doc, err := ingestionSvc.RegisterDocument(ctx, nb.ID, filePath)
	if err != nil {
		return fmt.Errorf("register document: %w", err)
	}

	chunkCount, err := ingestionSvc.ProcessRegisteredDocument(ctx, doc.ID, nb.Name)
	if err != nil {
		return fmt.Errorf("process document: %w", err)
	}

	fmt.Printf("ingestion complete: notebook=%s document=%s chunks=%d vec_enabled=%t\n", nb.ID, doc.ID, chunkCount, vecAvailable)

	if strings.TrimSpace(query) == "" {
		return nil
	}

	retrievalSvc := retrieval.NewService(database)
	results, err := retrievalSvc.SearchKeyword(ctx, nb.ID, query, 5)
	if err != nil {
		return fmt.Errorf("run retrieval query: %w", err)
	}

	fmt.Printf("retrieval results for query %q: %d rows\n", query, len(results))
	for i, chunk := range results {
		fmt.Printf("%d. chunk_id=%s idx=%d chapter=%s\n", i+1, chunk.ID, chunk.ChunkIndex, chunk.ChapterName)
	}

	return nil
}

func envBool(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes"
}

func runReviewSmoke(database *db.Database, notebookName string, cardCount int) error {
	if cardCount <= 0 {
		cardCount = 3
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	now := time.Now().UTC()
	nb := &db.Notebook{
		ID:          uuid.NewString(),
		Name:        notebookName,
		Description: "Sprint 3 review smoke run",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := database.DB.WithContext(ctx).Create(nb).Error; err != nil {
		return fmt.Errorf("create notebook: %w", err)
	}

	seedDoc := &db.Document{
		ID:         uuid.NewString(),
		NotebookID: nb.ID,
		Filename:   "sprint3-seed.txt",
		FilePath:   "sprint3-seed.txt",
		FileHash:   uuid.NewString(),
		Status:     "ready",
		CreatedAt:  now,
	}
	if err := database.DB.WithContext(ctx).Create(seedDoc).Error; err != nil {
		return fmt.Errorf("create seed document: %w", err)
	}

	seedChunk := &db.Chunk{
		ID:            uuid.NewString(),
		DocumentID:    seedDoc.ID,
		NotebookID:    nb.ID,
		ChapterName:   "General",
		ChunkIndex:    0,
		Content:       "Sprint 3 seed content for FSRS review workflow.",
		TaggedContent: "[" + nb.Name + " - General] Sprint 3 seed content for FSRS review workflow.",
		TokenCount:    10,
		CreatedAt:     now,
	}
	if err := database.DB.WithContext(ctx).Create(seedChunk).Error; err != nil {
		return fmt.Errorf("create seed chunk: %w", err)
	}

	for i := 0; i < cardCount; i++ {
		card := &db.Flashcard{
			ID:             uuid.NewString(),
			ChunkID:        seedChunk.ID,
			NotebookID:     nb.ID,
			Question:       fmt.Sprintf("Q%d: What is Sprint 3 review quality metric?", i+1),
			Answer:         "Stability, difficulty, retrievability and session telemetry.",
			Source:         "ai",
			Stability:      0.4,
			Difficulty:     5,
			Retrievability: 1,
			State:          "new",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := database.DB.WithContext(ctx).Create(card).Error; err != nil {
			return fmt.Errorf("create flashcard %d: %w", i+1, err)
		}
	}

	syncService := syncsvc.NewService(database)
	fsrsService := fsrs.NewService(database, syncService)
	schedulerService := scheduler.NewService(database)

	dueCards, err := schedulerService.NextDueCards(ctx, nb.ID, cardCount)
	if err != nil {
		return fmt.Errorf("load due cards: %w", err)
	}
	if len(dueCards) == 0 {
		return fmt.Errorf("no due cards available for review smoke")
	}

	ratings := []int{fsrs.RatingGood, fsrs.RatingEasy, fsrs.RatingHard, fsrs.RatingAgain}
	correct := 0
	totalTimeMs := 0
	sessionStart := time.Now().UTC()

	for i, card := range dueCards {
		rating := ratings[i%len(ratings)]
		if rating >= fsrs.RatingGood {
			correct++
		}
		timeTaken := 2500 + (i * 300)
		totalTimeMs += timeTaken

		if _, err := fsrsService.ReviewCard(ctx, fsrs.ReviewInput{
			FlashcardID:  card.ID,
			NotebookID:   nb.ID,
			NotebookName: nb.Name,
			Rating:       rating,
			TimeTakenMs:  timeTaken,
		}); err != nil {
			return fmt.Errorf("review card %s: %w", card.ID, err)
		}
	}

	sessionEnd := time.Now().UTC()
	if err := fsrsService.CompleteSession(ctx, fsrs.SessionSummary{
		NotebookID:         nb.ID,
		NotebookName:       nb.Name,
		StartedAt:          sessionStart,
		EndedAt:            sessionEnd,
		FlashcardsReviewed: len(dueCards),
		CorrectRecallCount: correct,
		TotalTimeTakenMS:   totalTimeMs,
		EmitTelemetry:      true,
	}); err != nil {
		return fmt.Errorf("complete review session: %w", err)
	}

	var reviewLogCount int64
	if err := database.DB.WithContext(ctx).Raw("SELECT COUNT(1) FROM review_logs").Scan(&reviewLogCount).Error; err != nil {
		return fmt.Errorf("count review logs: %w", err)
	}

	var syncQueueCount int64
	if err := database.DB.WithContext(ctx).Raw("SELECT COUNT(1) FROM sync_queue").Scan(&syncQueueCount).Error; err != nil {
		return fmt.Errorf("count sync queue rows: %w", err)
	}

	fmt.Printf("review smoke complete: notebook=%s reviewed=%d review_logs=%d sync_events=%d\n", nb.ID, len(dueCards), reviewLogCount, syncQueueCount)
	return nil
}
