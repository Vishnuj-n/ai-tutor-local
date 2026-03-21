package retrieval

import (
	"context"
	"fmt"

	"ai-tutor-local/internal/db"

	"gorm.io/gorm"
)

// Service provides baseline FTS retrieval operations for Sprint 2.
type Service struct {
	db *gorm.DB
}

func NewService(database *db.Database) *Service {
	return &Service{db: database.DB}
}

// SearchKeyword returns notebook-scoped chunks ranked by FTS5 BM25 score.
func (s *Service) SearchKeyword(ctx context.Context, notebookID, query string, limit int) ([]db.Chunk, error) {
	if limit <= 0 {
		limit = 10
	}

	rows := make([]db.Chunk, 0, limit)
	err := s.db.WithContext(ctx).Raw(`
SELECT c.*
FROM chunks_fts
JOIN chunks c ON c.rowid = chunks_fts.rowid
WHERE chunks_fts MATCH ?
  AND c.notebook_id = ?
ORDER BY bm25(chunks_fts)
LIMIT ?;
`, query, notebookID, limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("fts search failed: %w", err)
	}

	return rows, nil
}
