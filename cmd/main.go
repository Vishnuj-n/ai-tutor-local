package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-tutor-local/internal/db"
	"ai-tutor-local/internal/embedding"
	"ai-tutor-local/internal/ingestion"
	"ai-tutor-local/internal/retrieval"

	"github.com/google/uuid"
)

func main() {
	ingestFile := flag.String("ingest-file", "", "Absolute or relative file path to ingest (supports text and PDF)")
	notebookName := flag.String("notebook", "Sprint2 Notebook", "Notebook name for ingestion smoke run")
	query := flag.String("query", "", "Optional keyword query to run after ingestion")
	strictVec := flag.Bool("strict-vec", false, "Fail startup if sqlite-vec (vec0) is unavailable")
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

	fmt.Println("ai-tutor-local Sprint 2 baseline ready")
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
		ingestionSvc.SetEmbedder(embedding.NewClient("http://localhost:11434"))
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
