package taskengine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-tutor-local/internal/db"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	TaskTypeRead             = "READ"
	TaskTypeReviewFlashcards = "REVIEW_FLASHCARDS"
	TaskTypeTakeQuiz         = "TAKE_QUIZ"
)

type TaskBoardItem struct {
	ID           string    `json:"id"`
	NotebookID   string    `json:"notebook_id"`
	TopicID      string    `json:"topic_id,omitempty"`
	TaskType     string    `json:"task_type"`
	TargetType   string    `json:"target_type"`
	TargetID     string    `json:"target_id"`
	Title        string    `json:"title"`
	Instructions string    `json:"instructions,omitempty"`
	Status       string    `json:"status"`
	DueDate      time.Time `json:"due_date"`
	Position     int       `json:"position"`
}

type TaskContext struct {
	TaskID       string `json:"task_id"`
	TaskType     string `json:"task_type"`
	NotebookID   string `json:"notebook_id"`
	Notebook     string `json:"notebook_name,omitempty"`
	DocumentID   string `json:"document_id,omitempty"`
	Document     string `json:"document_filename,omitempty"`
	DocumentPath string `json:"document_path,omitempty"`
	TopicID      string `json:"topic_id,omitempty"`
	TopicTitle   string `json:"topic_title,omitempty"`
	ChunkID      string `json:"chunk_id,omitempty"`
	StartPage    int    `json:"start_page"`
}

type Service struct {
	db *db.Database
}

func NewService(database *db.Database) *Service {
	return &Service{db: database}
}

func (s *Service) BuildLearningPath(ctx context.Context, notebookID, documentID string) (int, error) {
	if strings.TrimSpace(notebookID) == "" {
		return 0, fmt.Errorf("notebook_id is required")
	}

	topics, err := s.ensureTopics(ctx, notebookID, documentID)
	if err != nil {
		return 0, err
	}
	if len(topics) == 0 {
		return 0, nil
	}

	now := time.Now().UTC()
	tasks := make([]db.DailyTask, 0, len(topics)*3)
	position := 1
	for i, topic := range topics {
		dueBase := now.AddDate(0, 0, i)
		status := "locked"
		if i == 0 {
			status = "pending"
		}

		tasks = append(tasks,
			db.DailyTask{
				ID:           uuid.NewString(),
				NotebookID:   notebookID,
				TopicID:      &topic.ID,
				TaskType:     TaskTypeRead,
				TargetType:   "topic",
				TargetID:     topic.ID,
				Title:        "Read: " + topic.Title,
				Instructions: "Read the source section and extract key points.",
				Status:       status,
				DueDate:      dueBase,
				Position:     position,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			db.DailyTask{
				ID:           uuid.NewString(),
				NotebookID:   notebookID,
				TopicID:      &topic.ID,
				TaskType:     TaskTypeReviewFlashcards,
				TargetType:   "topic",
				TargetID:     topic.ID,
				Title:        "Review Flashcards: " + topic.Title,
				Instructions: "Review the generated flashcards and rate recall.",
				Status:       "locked",
				DueDate:      dueBase,
				Position:     position + 1,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			db.DailyTask{
				ID:           uuid.NewString(),
				NotebookID:   notebookID,
				TopicID:      &topic.ID,
				TaskType:     TaskTypeTakeQuiz,
				TargetType:   "topic",
				TargetID:     topic.ID,
				Title:        "Take Quiz: " + topic.Title,
				Instructions: "Attempt topic quiz to validate mastery.",
				Status:       "locked",
				DueDate:      dueBase,
				Position:     position + 2,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		)
		position += 3
	}

	if err := s.db.DB.WithContext(ctx).CreateInBatches(tasks, 100).Error; err != nil {
		return 0, fmt.Errorf("create daily tasks: %w", err)
	}

	return len(tasks), nil
}

func (s *Service) ListTodayTasks(ctx context.Context, limit int) ([]TaskBoardItem, error) {
	if limit <= 0 {
		limit = 50
	}

	rows := make([]TaskBoardItem, 0)
	err := s.db.DB.WithContext(ctx).Raw(`
SELECT id, notebook_id, COALESCE(topic_id, '') AS topic_id, task_type, target_type, target_id, title, instructions, status, due_date, position
FROM daily_tasks
WHERE status IN ('pending','locked')
ORDER BY due_date ASC, position ASC
LIMIT ?;
`, limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list today tasks: %w", err)
	}

	return rows, nil
}

func (s *Service) ResolveTaskContext(ctx context.Context, taskID string) (*TaskContext, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	type taskRow struct {
		ID         string  `gorm:"column:id"`
		NotebookID string  `gorm:"column:notebook_id"`
		TopicID    *string `gorm:"column:topic_id"`
		TaskType   string  `gorm:"column:task_type"`
		TargetType string  `gorm:"column:target_type"`
		TargetID   string  `gorm:"column:target_id"`
	}
	var task taskRow
	if err := s.db.DB.WithContext(ctx).
		Table("daily_tasks").
		Select("id, notebook_id, topic_id, task_type, target_type, target_id").
		Where("id = ?", taskID).
		Take(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("load task: %w", err)
	}

	ctxOut := &TaskContext{
		TaskID:     task.ID,
		TaskType:   task.TaskType,
		NotebookID: task.NotebookID,
		TopicID:    deref(task.TopicID),
		StartPage:  1,
	}

	var nb struct {
		Name string `gorm:"column:name"`
	}
	_ = s.db.DB.WithContext(ctx).Table("notebooks").Select("name").Where("id = ?", task.NotebookID).Take(&nb).Error
	ctxOut.Notebook = nb.Name

	if task.TopicID != nil && strings.TrimSpace(*task.TopicID) != "" {
		var tp struct {
			ID         string  `gorm:"column:id"`
			Title      string  `gorm:"column:title"`
			DocumentID *string `gorm:"column:document_id"`
		}
		if err := s.db.DB.WithContext(ctx).Table("topics").Select("id, title, document_id").Where("id = ?", *task.TopicID).Take(&tp).Error; err == nil {
			ctxOut.TopicTitle = tp.Title
			ctxOut.DocumentID = deref(tp.DocumentID)
		}
	}

	if ctxOut.DocumentID != "" {
		var doc struct {
			Filename string `gorm:"column:filename"`
			FilePath string `gorm:"column:file_path"`
		}
		if err := s.db.DB.WithContext(ctx).Table("documents").Select("filename, file_path").Where("id = ?", ctxOut.DocumentID).Take(&doc).Error; err == nil {
			ctxOut.Document = doc.Filename
			ctxOut.DocumentPath = doc.FilePath
		}
	}

	if task.TargetType == "chunk" {
		ctxOut.ChunkID = task.TargetID
	}

	return ctxOut, nil
}

func (s *Service) ensureTopics(ctx context.Context, notebookID, documentID string) ([]db.Topic, error) {
	existing := make([]db.Topic, 0)
	query := s.db.DB.WithContext(ctx).Where("notebook_id = ?", notebookID)
	if strings.TrimSpace(documentID) != "" {
		query = query.Where("document_id = ?", documentID)
	}
	if err := query.Order("sequence_order ASC, created_at ASC").Find(&existing).Error; err != nil {
		return nil, fmt.Errorf("load topics: %w", err)
	}
	if len(existing) > 0 {
		return existing, nil
	}

	type chapterRow struct {
		ChapterName string `gorm:"column:chapter_name"`
	}
	chapters := make([]chapterRow, 0)
	raw := s.db.DB.WithContext(ctx).
		Table("chunks").
		Select("chapter_name").
		Where("notebook_id = ?", notebookID)
	if strings.TrimSpace(documentID) != "" {
		raw = raw.Where("document_id = ?", documentID)
	}
	if err := raw.Group("chapter_name").Order("MIN(chunk_index) ASC").Scan(&chapters).Error; err != nil {
		return nil, fmt.Errorf("derive topics from chunks: %w", err)
	}

	now := time.Now().UTC()
	newTopics := make([]db.Topic, 0, len(chapters))
	for i, row := range chapters {
		title := strings.TrimSpace(row.ChapterName)
		if title == "" {
			title = fmt.Sprintf("Topic %d", i+1)
		}
		var docID *string
		if strings.TrimSpace(documentID) != "" {
			tmp := documentID
			docID = &tmp
		}
		newTopics = append(newTopics, db.Topic{
			ID:            uuid.NewString(),
			NotebookID:    notebookID,
			DocumentID:    docID,
			Title:         title,
			SourceHeading: row.ChapterName,
			SequenceOrder: i + 1,
			MasteryState:  "new",
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	if len(newTopics) == 0 {
		return nil, nil
	}
	if err := s.db.DB.WithContext(ctx).CreateInBatches(newTopics, 100).Error; err != nil {
		return nil, fmt.Errorf("create topics: %w", err)
	}
	return newTopics, nil
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
