//go:build sqlite_fts5

package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-tutor-local/internal/db"
	"ai-tutor-local/internal/ingestion"

	"github.com/google/uuid"
)

func TestIngestAndFTSRetrieveSmoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	schemaPath := resolveSchemaPath(t)
	samplePath := filepath.Join(tempDir, "sample.txt")

	sample := "Polity studies the Constitution and governance structure. Parliament makes laws. Federalism shapes center-state relations."
	if err := os.WriteFile(samplePath, []byte(sample), 0644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	database, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.RunSchemaMigrationsWithOptions(schemaPath, db.MigrationOptions{SkipVectorTable: true}); err != nil {
		t.Fatalf("run schema migrations: %v", err)
	}

	notebook := &db.Notebook{
		ID:          uuid.NewString(),
		Name:        "Polity",
		Description: "Smoke test notebook",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := database.DB.WithContext(ctx).Create(notebook).Error; err != nil {
		t.Fatalf("create notebook: %v", err)
	}

	ingestSvc := ingestion.NewService(database)
	ingestSvc.SetVectorStoreEnabled(false)

	doc, err := ingestSvc.RegisterDocument(ctx, notebook.ID, samplePath)
	if err != nil {
		t.Fatalf("register document: %v", err)
	}

	chunkCount, err := ingestSvc.ProcessRegisteredDocument(ctx, doc.ID, notebook.Name)
	if err != nil {
		t.Fatalf("process document: %v", err)
	}
	if chunkCount == 0 {
		t.Fatal("expected non-zero chunks")
	}

	retrievalSvc := NewService(database)
	results, err := retrievalSvc.SearchKeyword(ctx, notebook.ID, "Parliament", 5)
	if err != nil {
		t.Fatalf("fts search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one retrieval result")
	}
}

func resolveSchemaPath(t *testing.T) string {
	t.Helper()

	candidates := []string{
		filepath.Clean(filepath.Join("..", "..", "schema.sql")),
		"schema.sql",
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	t.Fatalf("schema.sql not found from package working directory")
	return ""
}
