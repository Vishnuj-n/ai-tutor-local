package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// NotebookQueries provides CRUD operations for notebooks.
type NotebookQueries struct {
	db *gorm.DB
}

func NewNotebookQueries(database *gorm.DB) *NotebookQueries {
	return &NotebookQueries{db: database}
}

func (q *NotebookQueries) Create(notebook *Notebook) error {
	return q.db.Create(notebook).Error
}

func (q *NotebookQueries) GetByID(id string) (*Notebook, error) {
	var notebook Notebook
	err := q.db.First(&notebook, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &notebook, nil
}

func (q *NotebookQueries) List() ([]Notebook, error) {
	var notebooks []Notebook
	err := q.db.Order("updated_at DESC").Find(&notebooks).Error
	return notebooks, err
}

func (q *NotebookQueries) Update(notebook *Notebook) error {
	return q.db.Save(notebook).Error
}

func (q *NotebookQueries) Delete(id string) error {
	return q.db.Delete(&Notebook{}, "id = ?", id).Error
}

// ChunkQueries provides operations for chunk storage/retrieval.
type ChunkQueries struct {
	db *gorm.DB
}

func NewChunkQueries(database *gorm.DB) *ChunkQueries {
	return &ChunkQueries{db: database}
}

func (q *ChunkQueries) CreateBatch(chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return q.db.CreateInBatches(chunks, 100).Error
}

func (q *ChunkQueries) ListByDocumentID(documentID string) ([]Chunk, error) {
	var chunks []Chunk
	err := q.db.Where("document_id = ?", documentID).Order("chunk_index ASC").Find(&chunks).Error
	return chunks, err
}

func (q *ChunkQueries) ListByNotebookID(notebookID string) ([]Chunk, error) {
	var chunks []Chunk
	err := q.db.Where("notebook_id = ?", notebookID).Order("created_at DESC").Find(&chunks).Error
	return chunks, err
}

// EmbeddingQueries provides operations for sqlite-vec backed embeddings.
type EmbeddingQueries struct {
	db *gorm.DB
}

func NewEmbeddingQueries(database *gorm.DB) *EmbeddingQueries {
	return &EmbeddingQueries{db: database}
}

func (q *EmbeddingQueries) UpsertBatchByChunkID(ctx context.Context, chunks []Chunk, vectors [][]float32) error {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) != len(vectors) {
		return fmt.Errorf("chunks/vector length mismatch: %d/%d", len(chunks), len(vectors))
	}

	return q.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range chunks {
			if err := upsertEmbeddingByChunkID(ctx, tx, chunks[i].ID, vectors[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertEmbeddingByChunkID(ctx context.Context, tx *gorm.DB, chunkID string, embedding []float32) error {
	var row struct {
		RowID int64 `gorm:"column:rowid"`
	}

	if err := tx.WithContext(ctx).Raw("SELECT rowid FROM chunks WHERE id = ?", chunkID).Scan(&row).Error; err != nil {
		return fmt.Errorf("lookup chunk rowid: %w", err)
	}
	if row.RowID == 0 {
		return fmt.Errorf("chunk rowid not found for chunk_id=%s", chunkID)
	}

	vectorJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding vector: %w", err)
	}

	if err := tx.WithContext(ctx).Exec(`
INSERT INTO embeddings(chunk_rowid, embedding)
VALUES (?, ?)
ON CONFLICT(chunk_rowid) DO UPDATE SET embedding=excluded.embedding;
`, row.RowID, string(vectorJSON)).Error; err != nil {
		return fmt.Errorf("upsert embedding row: %w", err)
	}

	return nil
}

// FlashcardQueries provides operations for spaced repetition cards.
type FlashcardQueries struct {
	db *gorm.DB
}

func NewFlashcardQueries(database *gorm.DB) *FlashcardQueries {
	return &FlashcardQueries{db: database}
}

func (q *FlashcardQueries) CreateBatch(cards []Flashcard) error {
	if len(cards) == 0 {
		return nil
	}
	return q.db.CreateInBatches(cards, 100).Error
}

func (q *FlashcardQueries) ListDueByNotebookID(notebookID string, now time.Time) ([]Flashcard, error) {
	var cards []Flashcard
	err := q.db.Where("notebook_id = ? AND (due_date IS NULL OR due_date <= ?)", notebookID, now).
		Order("due_date ASC, state ASC").
		Find(&cards).Error
	return cards, err
}

func (q *FlashcardQueries) Update(card *Flashcard) error {
	return q.db.Save(card).Error
}

// ReviewLogQueries provides operations for review history.
type ReviewLogQueries struct {
	db *gorm.DB
}

func NewReviewLogQueries(database *gorm.DB) *ReviewLogQueries {
	return &ReviewLogQueries{db: database}
}

func (q *ReviewLogQueries) Create(log *ReviewLog) error {
	return q.db.Create(log).Error
}

func (q *ReviewLogQueries) SumTimeTakenMsBetween(notebookID string, start, end time.Time) (int64, error) {
	var total int64
	err := q.db.Table("review_logs").
		Select("COALESCE(SUM(review_logs.time_taken_ms), 0)").
		Joins("JOIN flashcards ON flashcards.id = review_logs.flashcard_id").
		Where("flashcards.notebook_id = ? AND review_logs.reviewed_at BETWEEN ? AND ?", notebookID, start, end).
		Scan(&total).Error
	return total, err
}

// SyncQueueQueries provides operations for offline-first sync queue.
type SyncQueueQueries struct {
	db *gorm.DB
}

func NewSyncQueueQueries(database *gorm.DB) *SyncQueueQueries {
	return &SyncQueueQueries{db: database}
}

func (q *SyncQueueQueries) Enqueue(item *SyncQueueItem) error {
	return q.db.Create(item).Error
}

func (q *SyncQueueQueries) ListPending(limit int) ([]SyncQueueItem, error) {
	var items []SyncQueueItem
	err := q.db.Where("status = ?", "pending").Order("created_at ASC").Limit(limit).Find(&items).Error
	return items, err
}

func (q *SyncQueueQueries) MarkAttempt(id, status string) error {
	return q.db.Model(&SyncQueueItem{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       status,
			"attempts":     gorm.Expr("attempts + 1"),
			"last_attempt": time.Now().UTC(),
		}).Error
}

// StudentConfigQueries provides operations for app configuration.
type StudentConfigQueries struct {
	db *gorm.DB
}

func NewStudentConfigQueries(database *gorm.DB) *StudentConfigQueries {
	return &StudentConfigQueries{db: database}
}

func (q *StudentConfigQueries) Set(key, value string) error {
	return q.db.Save(&StudentConfig{Key: key, Value: value}).Error
}

func (q *StudentConfigQueries) Get(key string) (string, error) {
	var cfg StudentConfig
	err := q.db.First(&cfg, "key = ?", key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return cfg.Value, nil
}
