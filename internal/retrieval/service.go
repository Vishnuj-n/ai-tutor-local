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

	type chunkRow struct {
		ID            string `gorm:"column:id"`
		DocumentID    string `gorm:"column:document_id"`
		NotebookID    string `gorm:"column:notebook_id"`
		ChapterName   string `gorm:"column:chapter_name"`
		ChunkIndex    int    `gorm:"column:chunk_index"`
		Content       string `gorm:"column:content"`
		TaggedContent string `gorm:"column:tagged_content"`
		TokenCount    int    `gorm:"column:token_count"`
	}

	rawRows := make([]chunkRow, 0, limit)
	err := s.db.WithContext(ctx).Raw(`
SELECT c.id, c.document_id, c.notebook_id, c.chapter_name, c.chunk_index, c.content, c.tagged_content, c.token_count
FROM chunks_fts
JOIN chunks c ON c.rowid = chunks_fts.rowid
WHERE chunks_fts MATCH ?
  AND c.notebook_id = ?
ORDER BY bm25(chunks_fts)
LIMIT ?;
`, query, notebookID, limit).Scan(&rawRows).Error
	if err != nil {
		return nil, fmt.Errorf("fts search failed: %w", err)
	}

	rows := make([]db.Chunk, 0, len(rawRows))
	for _, r := range rawRows {
		rows = append(rows, db.Chunk{
			ID:            r.ID,
			DocumentID:    r.DocumentID,
			NotebookID:    r.NotebookID,
			ChapterName:   r.ChapterName,
			ChunkIndex:    r.ChunkIndex,
			Content:       r.Content,
			TaggedContent: r.TaggedContent,
			TokenCount:    r.TokenCount,
		})
	}

	return rows, nil
}
