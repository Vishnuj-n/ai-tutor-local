package scheduler

import (
	"context"
	"fmt"
	"time"

	"ai-tutor-local/internal/db"
)

// DueCard is the queue shape used by review UI/application layers.
type DueCard struct {
	ID         string
	NotebookID string
	ChunkID    string
	Question   string
	Answer     string
	State      string
	DueDate    *time.Time
	Reps       int
	Lapses     int
}

// Service provides due-card scheduling helpers for Sprint 3.
type Service struct {
	database *db.Database
}

func NewService(database *db.Database) *Service {
	return &Service{database: database}
}

// NextDueCards returns cards due now (or unscheduled cards) for a notebook.
func (s *Service) NextDueCards(ctx context.Context, notebookID string, limit int) ([]DueCard, error) {
	if limit <= 0 {
		limit = 20
	}

	type dueRow struct {
		ID         string     `gorm:"column:id"`
		NotebookID string     `gorm:"column:notebook_id"`
		ChunkID    string     `gorm:"column:chunk_id"`
		Question   string     `gorm:"column:question"`
		Answer     string     `gorm:"column:answer"`
		State      string     `gorm:"column:state"`
		DueDate    *time.Time `gorm:"column:due_date"`
		Reps       int        `gorm:"column:reps"`
		Lapses     int        `gorm:"column:lapses"`
	}

	rows := make([]dueRow, 0, limit)
	err := s.database.DB.WithContext(ctx).Raw(`
SELECT id, notebook_id, chunk_id, question, answer, state, due_date, reps, lapses
FROM flashcards
WHERE notebook_id = ?
  AND (due_date IS NULL OR due_date <= ?)
ORDER BY COALESCE(due_date, '1970-01-01T00:00:00Z') ASC
LIMIT ?;
`, notebookID, time.Now().UTC(), limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query due cards: %w", err)
	}

	out := make([]DueCard, 0, len(rows))
	for _, r := range rows {
		out = append(out, DueCard(r))
	}

	return out, nil
}

// SessionDueCount returns count of cards currently due for quick dashboard metrics.
func (s *Service) SessionDueCount(ctx context.Context, notebookID string) (int64, error) {
	var count int64
	err := s.database.DB.WithContext(ctx).Raw(`
SELECT COUNT(1)
FROM flashcards
WHERE notebook_id = ?
  AND (due_date IS NULL OR due_date <= ?);
`, notebookID, time.Now().UTC()).Scan(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count due cards: %w", err)
	}
	return count, nil
}
