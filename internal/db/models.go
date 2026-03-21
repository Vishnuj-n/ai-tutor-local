package db

import "time"

// Notebook represents a subject notebook (e.g., "Polity", "Economics")
type Notebook struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null;index" json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `gorm:"autoCreateTime:milli" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime:milli" json:"updated_at"`

	// Relationships
	Documents   []Document    `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"documents,omitempty"`
	Flashcards  []Flashcard   `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"flashcards,omitempty"`
	Chunks      []Chunk       `gorm:"foreignKey:NotebookID" json:"chunks,omitempty"`
	QuizSession []QuizSession `gorm:"foreignKey:NotebookID" json:"quiz_sessions,omitempty"`
}

// Document represents a PDF or text file uploaded to a notebook
type Document struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	NotebookID string    `gorm:"not null;index" json:"notebook_id"`
	Filename   string    `gorm:"not null" json:"filename"`
	FilePath   string    `gorm:"not null" json:"file_path"`
	FileHash   string    `gorm:"not null;uniqueIndex:idx_file_hash" json:"file_hash"` // SHA256
	PageCount  int       `json:"page_count"`
	Status     string    `gorm:"default:'pending';index" json:"status"` // pending | processing | ready | error
	ErrorMsg   string    `json:"error_msg,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime:milli" json:"created_at"`

	// Relationships
	Notebook Notebook `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"notebook,omitempty"`
	Chunks   []Chunk  `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"chunks,omitempty"`
}

// Chunk represents a semantic chunk from a document
type Chunk struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	DocumentID    string    `gorm:"not null;index" json:"document_id"`
	NotebookID    string    `gorm:"not null;index" json:"notebook_id"`
	ChapterName   string    `json:"chapter_name"` // Heading this chunk belongs to
	ChunkIndex    int       `gorm:"not null" json:"chunk_index"`
	Content       string    `gorm:"type:text;not null" json:"content"`        // Raw chunk text
	TaggedContent string    `gorm:"type:text;not null" json:"tagged_content"` // '[Notebook - Chapter] content'
	TokenCount    int       `json:"token_count"`
	CreatedAt     time.Time `gorm:"autoCreateTime:milli" json:"created_at"`

	// Relationships
	Document   Document    `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document,omitempty"`
	Flashcards []Flashcard `gorm:"foreignKey:ChunkID" json:"flashcards,omitempty"`
}

// Flashcard represents a question-answer card for spaced repetition
type Flashcard struct {
	ID         string `gorm:"primaryKey" json:"id"`
	ChunkID    string `gorm:"not null" json:"chunk_id"`
	NotebookID string `gorm:"not null;index" json:"notebook_id"`
	Question   string `gorm:"type:text;not null" json:"question"`
	Answer     string `gorm:"type:text;not null" json:"answer"`
	Source     string `gorm:"default:'ai'" json:"source"` // 'ai' or 'user'

	// FSRS algorithm fields
	Stability      float32    `gorm:"default:0.0" json:"stability"`
	Difficulty     float32    `gorm:"default:0.0" json:"difficulty"`
	Retrievability float32    `gorm:"default:1.0" json:"retrievability"`
	DueDate        *time.Time `json:"due_date"` // NULL = new card not yet reviewed
	Reps           int        `gorm:"default:0" json:"reps"`
	Lapses         int        `gorm:"default:0" json:"lapses"`
	State          string     `gorm:"default:'new';index" json:"state"` // new | learning | review | relearning
	LastReview     *time.Time `json:"last_review"`

	CreatedAt time.Time `gorm:"autoCreateTime:milli" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:milli" json:"updated_at"`

	// Relationships
	Chunk      Chunk       `gorm:"foreignKey:ChunkID;constraint:OnDelete:CASCADE" json:"chunk,omitempty"`
	ReviewLogs []ReviewLog `gorm:"foreignKey:FlashcardID;constraint:OnDelete:CASCADE" json:"review_logs,omitempty"`
}

// ReviewLog records a single spaced repetition review
type ReviewLog struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	FlashcardID string    `gorm:"not null;index" json:"flashcard_id"`
	Rating      int       `gorm:"not null" json:"rating"` // 1=Again  2=Hard  3=Good  4=Easy
	ReviewedAt  time.Time `gorm:"not null;index:idx_reviewed_at,sort:desc" json:"reviewed_at"`
	TimeTakenMs int       `json:"time_taken_ms"` // milliseconds

	// Relationships
	Flashcard Flashcard `gorm:"foreignKey:FlashcardID;constraint:OnDelete:CASCADE" json:"flashcard,omitempty"`
}

// QuizSession records a quiz attempt
type QuizSession struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	NotebookID  string    `gorm:"not null;index" json:"notebook_id"`
	TopicName   string    `json:"topic_name"`
	Score       int       `gorm:"not null" json:"score"`
	Total       int       `gorm:"not null" json:"total"`
	AccuracyPct float32   `gorm:"not null" json:"accuracy_pct"`
	StartedAt   time.Time `gorm:"not null" json:"started_at"`
	CompletedAt time.Time `gorm:"not null" json:"completed_at"`
	Synced      int       `gorm:"default:0;index" json:"synced"` // 0 = pending, 1 = synced
}

// StudySession records any study activity (flashcard review, reading, search)
type StudySession struct {
	ID                  string    `gorm:"primaryKey" json:"id"`
	NotebookID          string    `json:"notebook_id"`
	ActivityType        string    `gorm:"not null" json:"activity_type"` // flashcard | quiz | reading | search
	TimeSpentSeconds    int       `gorm:"not null" json:"time_spent_seconds"`
	FlashcardsCompleted int       `gorm:"default:0" json:"flashcards_completed"`
	AccuracyPct         *float32  `json:"accuracy_pct"`
	StartedAt           time.Time `gorm:"not null" json:"started_at"`
	EndedAt             time.Time `gorm:"not null" json:"ended_at"`
	Synced              int       `gorm:"default:0;index" json:"synced"` // 0 = pending, 1 = synced
}

// SyncQueueItem represents a pending analytics event to be synced to cloud
type SyncQueueItem struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	Payload     string     `gorm:"type:text;not null" json:"payload"` // JSON-encoded analytics event
	CreatedAt   time.Time  `gorm:"autoCreateTime:milli;index:idx_created,sort:desc" json:"created_at"`
	Attempts    int        `gorm:"default:0" json:"attempts"`
	LastAttempt *time.Time `json:"last_attempt"`
	Status      string     `gorm:"default:'pending';index" json:"status"` // pending | sent | failed
}

func (SyncQueueItem) TableName() string {
	return "sync_queue"
}

// StudentConfig stores key-value configuration for the student
type StudentConfig struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `gorm:"type:text;not null" json:"value"`
}

func (StudentConfig) TableName() string {
	return "student_config"
}

// ConfigKeys for student_config table
const (
	ConfigStudentID     = "student_id"
	ConfigName          = "name"
	ConfigUSN           = "usn"
	ConfigClassID       = "class_id"
	ConfigClassCode     = "class_code"
	ConfigLLMMode       = "llm_mode"       // 'local' | 'api'
	ConfigAPIKey        = "api_key"        // encrypted
	ConfigAPIProvider   = "api_provider"   // 'openai' | 'gemini' | 'anthropic'
	ConfigEmbeddingMode = "embedding_mode" // always 'ollama'
	ConfigOllamaURL     = "local_ollama_url"
)
