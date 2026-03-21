package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-tutor-local/internal/db"

	"github.com/google/uuid"
)

// Service orchestrates document ingestion workflow.
type Service struct {
	db      *db.Database
	chunker *Chunker
}

func NewService(database *db.Database) *Service {
	return &Service{
		db:      database,
		chunker: NewChunker(400, 50),
	}
}

// RegisterDocument records a document before async ingestion starts.
func (s *Service) RegisterDocument(ctx context.Context, notebookID, filePath string) (*db.Document, error) {
	_ = ctx

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	hash := sha256.Sum256(content)
	doc := &db.Document{
		ID:         uuid.NewString(),
		NotebookID: notebookID,
		Filename:   filepath.Base(filePath),
		FilePath:   filePath,
		FileHash:   hex.EncodeToString(hash[:]),
		Status:     "pending",
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.db.DB.Create(doc).Error; err != nil {
		return nil, fmt.Errorf("insert document: %w", err)
	}

	return doc, nil
}
