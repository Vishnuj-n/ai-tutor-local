package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-tutor-local/internal/db"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
	"gorm.io/gorm"
)

// TextEmbedder defines the embedding contract needed by ingestion.
type TextEmbedder interface {
	EmbedText(ctx context.Context, texts []string) ([][]float32, error)
}

// Service orchestrates document ingestion workflow.
type Service struct {
	db                 *db.Database
	chunker            *Chunker
	embedder           TextEmbedder
	embeddingQueries   *db.EmbeddingQueries
	vectorStoreEnabled bool
}

func NewService(database *db.Database) *Service {
	return &Service{
		db:                 database,
		chunker:            NewChunker(400, 50),
		embeddingQueries:   db.NewEmbeddingQueries(database.DB),
		vectorStoreEnabled: false,
	}
}

// SetEmbedder wires an embedding client for end-to-end ingestion.
func (s *Service) SetEmbedder(embedder TextEmbedder) {
	s.embedder = embedder
}

// SetVectorStoreEnabled toggles sqlite-vec persistence behavior.
func (s *Service) SetVectorStoreEnabled(enabled bool) {
	s.vectorStoreEnabled = enabled
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

// ProcessRegisteredDocument reads a registered file, chunks it, and persists chunks.
func (s *Service) ProcessRegisteredDocument(ctx context.Context, documentID, notebookName string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var doc struct {
		ID         string `gorm:"column:id"`
		NotebookID string `gorm:"column:notebook_id"`
		FilePath   string `gorm:"column:file_path"`
	}
	if err := s.db.DB.WithContext(ctx).
		Model(&db.Document{}).
		Select("id", "notebook_id", "file_path").
		Where("id = ?", documentID).
		Take(&doc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("document not found: %s", documentID)
		}
		return 0, fmt.Errorf("load document: %w", err)
	}

	if err := s.db.DB.WithContext(ctx).Model(&db.Document{}).Where("id = ?", documentID).
		Updates(map[string]interface{}{"status": "processing", "error_msg": ""}).Error; err != nil {
		return 0, fmt.Errorf("mark document processing: %w", err)
	}

	rawText, err := extractText(doc.FilePath)
	if err != nil {
		_ = s.markDocumentError(ctx, documentID, fmt.Sprintf("extract text: %v", err))
		return 0, fmt.Errorf("extract text: %w", err)
	}

	sections := splitIntoHeadingSections(rawText)
	chunks := make([]ChunkResult, 0)
	nextChunkIndex := 0
	for _, section := range sections {
		sectionChunks := s.chunker.ChunkText(notebookName, section.Heading, section.Content)
		for _, chunk := range sectionChunks {
			chunk.ChunkIndex = nextChunkIndex
			nextChunkIndex++
			chunks = append(chunks, chunk)
		}
	}

	if len(chunks) == 0 {
		_ = s.markDocumentError(ctx, documentID, "no text content found for chunking")
		return 0, fmt.Errorf("no chunkable text found in document")
	}

	chunkRows := make([]db.Chunk, 0, len(chunks))
	now := time.Now().UTC()
	for _, c := range chunks {
		chunkRows = append(chunkRows, db.Chunk{
			ID:            uuid.NewString(),
			DocumentID:    doc.ID,
			NotebookID:    doc.NotebookID,
			ChapterName:   c.ChapterName,
			ChunkIndex:    c.ChunkIndex,
			Content:       c.Content,
			TaggedContent: c.TaggedContent,
			TokenCount:    c.TokenCount,
			CreatedAt:     now,
		})
	}

	vectors := make([][]float32, 0)
	if s.vectorStoreEnabled {
		if s.embedder == nil {
			_ = s.markDocumentError(ctx, documentID, "vector store enabled but embedder is nil")
			return 0, fmt.Errorf("vector store enabled but embedder is nil")
		}

		chunkTexts := make([]string, 0, len(chunkRows))
		for _, row := range chunkRows {
			chunkTexts = append(chunkTexts, row.TaggedContent)
		}
		embeddings, embedErr := s.embedder.EmbedText(ctx, chunkTexts)
		if embedErr != nil {
			_ = s.markDocumentError(ctx, documentID, fmt.Sprintf("embedding failed: %v", embedErr))
			return 0, fmt.Errorf("embed chunks: %w", embedErr)
		}
		if len(embeddings) != len(chunkRows) {
			_ = s.markDocumentError(ctx, documentID, "embedding count mismatch")
			return 0, fmt.Errorf("embedding count mismatch: got %d want %d", len(embeddings), len(chunkRows))
		}
		vectors = embeddings
	}

	if err := s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(chunkRows, 100).Error; err != nil {
			return fmt.Errorf("insert chunks: %w", err)
		}

		if s.vectorStoreEnabled {
			if err := s.embeddingQueries.UpsertBatchByChunkID(ctx, chunkRows, vectors); err != nil {
				return fmt.Errorf("persist embeddings: %w", err)
			}
		}

		if err := tx.Model(&db.Document{}).Where("id = ?", documentID).
			Updates(map[string]interface{}{"status": "ready", "error_msg": ""}).Error; err != nil {
			return fmt.Errorf("mark document ready: %w", err)
		}
		return nil
	}); err != nil {
		_ = s.markDocumentError(ctx, documentID, err.Error())
		return 0, err
	}

	return len(chunkRows), nil
}

func (s *Service) markDocumentError(ctx context.Context, documentID, message string) error {
	return s.db.DB.WithContext(ctx).Model(&db.Document{}).Where("id = ?", documentID).
		Updates(map[string]interface{}{"status": "error", "error_msg": message}).Error
}

func extractText(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".pdf" {
		return extractTextFromPDF(filePath)
	}

	b, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	text := strings.TrimSpace(string(b))
	if text == "" {
		return "", fmt.Errorf("empty text content")
	}

	return text, nil
}

func extractTextFromPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	b, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}

	raw, err := io.ReadAll(b)
	if err != nil {
		return "", fmt.Errorf("decode pdf text stream: %w", err)
	}

	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", fmt.Errorf("pdf has no extractable text")
	}

	return text, nil
}

type headingSection struct {
	Heading string
	Content string
}

func splitIntoHeadingSections(rawText string) []headingSection {
	normalized := strings.ReplaceAll(rawText, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	sections := make([]headingSection, 0)

	currentHeading := "General"
	buffer := make([]string, 0, 128)

	flush := func() {
		content := strings.TrimSpace(strings.Join(buffer, "\n"))
		if content == "" {
			buffer = buffer[:0]
			return
		}
		sections = append(sections, headingSection{
			Heading: currentHeading,
			Content: content,
		})
		buffer = buffer[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(buffer) > 0 {
				buffer = append(buffer, "")
			}
			continue
		}

		if heading, ok := detectHeadingLine(trimmed); ok {
			flush()
			currentHeading = heading
			continue
		}

		buffer = append(buffer, trimmed)
	}

	flush()

	if len(sections) == 0 {
		fallback := strings.TrimSpace(rawText)
		if fallback == "" {
			return nil
		}
		return []headingSection{{Heading: "General", Content: fallback}}
	}

	return sections
}

func detectHeadingLine(line string) (string, bool) {
	if strings.HasPrefix(line, "#") {
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if len(heading) >= 3 {
			return heading, true
		}
	}

	if strings.HasSuffix(line, ":") && len(line) <= 90 {
		heading := strings.TrimSuffix(line, ":")
		heading = strings.TrimSpace(heading)
		if len(heading) >= 3 {
			return heading, true
		}
	}

	if looksLikeNumberedHeading(line) {
		return strings.TrimSpace(line), true
	}

	if isMostlyUpperHeading(line) {
		return strings.TrimSpace(line), true
	}

	return "", false
}

func looksLikeNumberedHeading(line string) bool {
	if len(line) < 3 || len(line) > 90 {
		return false
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return false
	}

	prefix := strings.TrimSpace(parts[0])
	prefix = strings.TrimSuffix(prefix, ".")
	prefix = strings.TrimSuffix(prefix, ")")
	if prefix == "" {
		return false
	}

	if _, err := strconv.Atoi(prefix); err == nil {
		return true
	}

	roman := strings.ToUpper(prefix)
	for _, ch := range roman {
		if !strings.ContainsRune("IVXLCDM", ch) {
			return false
		}
	}

	return true
}

func isMostlyUpperHeading(line string) bool {
	if len(line) < 4 || len(line) > 80 {
		return false
	}

	letters := 0
	uppercase := 0
	for _, ch := range line {
		if ch >= 'A' && ch <= 'Z' {
			letters++
			uppercase++
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			letters++
		}
	}

	if letters < 4 {
		return false
	}

	ratio := float64(uppercase) / float64(letters)
	return ratio >= 0.85
}
