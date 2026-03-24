package db

import (
	"context"
	"database/sql"
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

func (q *SyncQueueQueries) ListRetryable(limit int) ([]SyncQueueItem, error) {
	type syncQueueRow struct {
		ID          string         `gorm:"column:id"`
		Payload     string         `gorm:"column:payload"`
		CreatedAt   sql.NullString `gorm:"column:created_at"`
		Attempts    int            `gorm:"column:attempts"`
		LastAttempt sql.NullString `gorm:"column:last_attempt"`
		Status      string         `gorm:"column:status"`
	}

	rows := make([]syncQueueRow, 0)
	err := q.db.
		Table("sync_queue").
		Select("id, payload, CAST(created_at AS TEXT) AS created_at, attempts, CAST(last_attempt AS TEXT) AS last_attempt, status").
		Where("status IN ?", []string{"pending", "failed"}).
		Order("created_at ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]SyncQueueItem, 0, len(rows))
	for _, row := range rows {
		if !row.CreatedAt.Valid {
			return nil, fmt.Errorf("missing created_at for sync_queue id=%s", row.ID)
		}

		createdAt, err := parseSQLiteTimeString(row.CreatedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse created_at for sync_queue id=%s: %w", row.ID, err)
		}

		var lastAttempt *time.Time
		if row.LastAttempt.Valid {
			parsedLastAttempt, err := parseSQLiteTimeString(row.LastAttempt.String)
			if err != nil {
				return nil, fmt.Errorf("parse last_attempt for sync_queue id=%s: %w", row.ID, err)
			}
			lastAttempt = &parsedLastAttempt
		}

		items = append(items, SyncQueueItem{
			ID:          row.ID,
			Payload:     row.Payload,
			CreatedAt:   createdAt,
			Attempts:    row.Attempts,
			LastAttempt: lastAttempt,
			Status:      row.Status,
		})
	}

	return items, nil
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

func (q *SyncQueueQueries) CountPendingAndFailed() (int64, error) {
	var count int64
	err := q.db.Model(&SyncQueueItem{}).
		Where("status IN ?", []string{"pending", "failed"}).
		Count(&count).Error
	return count, err
}

func (q *SyncQueueQueries) LastSuccessfulSyncAt() (*time.Time, error) {
	var row struct {
		LastAttempt sql.NullString `gorm:"column:last_attempt"`
	}
	err := q.db.
		Table("sync_queue").
		Select("CAST(last_attempt AS TEXT) AS last_attempt").
		Where("status = ?", "sent").
		Where("last_attempt IS NOT NULL").
		Order("last_attempt DESC").
		Limit(1).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if !row.LastAttempt.Valid {
		return nil, nil
	}

	parsed, err := parseSQLiteTimeString(row.LastAttempt.String)
	if err != nil {
		return nil, fmt.Errorf("parse last successful sync time: %w", err)
	}

	return &parsed, nil
}

func parseSQLiteTimeString(value string) (time.Time, error) {
	trimmed := value
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
	}

	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}

	return time.Time{}, fmt.Errorf("parse sqlite time %q: %w", trimmed, lastErr)
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
