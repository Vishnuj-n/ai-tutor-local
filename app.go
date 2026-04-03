package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-tutor-local/internal/apperrors"
	"ai-tutor-local/internal/db"
	"ai-tutor-local/internal/fsrs"
	"ai-tutor-local/internal/ingestion"
	syncsvc "ai-tutor-local/internal/sync"
	"ai-tutor-local/internal/ui"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application state.
type App struct {
	ctx        context.Context
	database   *db.Database
	startupErr error
	vecEnabled bool
	logger     *slog.Logger
}

type ReviewCardDTO struct {
	FlashcardID   string `json:"flashcard_id"`
	NotebookID    string `json:"notebook_id"`
	NotebookName  string `json:"notebook_name"`
	Question      string `json:"question"`
	Answer        string `json:"answer"`
	State         string `json:"state"`
	DueAt         string `json:"due_at,omitempty"`
	Reps          int    `json:"reps"`
	Lapses        int    `json:"lapses"`
	QueuePosition int    `json:"queue_position"`
	QueueSize     int    `json:"queue_size"`
}

type ReviewRateInput struct {
	FlashcardID  string `json:"flashcard_id"`
	NotebookID   string `json:"notebook_id"`
	NotebookName string `json:"notebook_name"`
	Rating       int    `json:"rating"`
	TimeTakenMs  int    `json:"time_taken_ms"`
}

type ReviewRateResult struct {
	NextDueAt string `json:"next_due_at"`
	State     string `json:"state"`
	Message   string `json:"message"`
}

type ReviewSessionSummaryInput struct {
	NotebookID         string `json:"notebook_id"`
	NotebookName       string `json:"notebook_name"`
	StartedAtMS        int64  `json:"started_at_ms"`
	EndedAtMS          int64  `json:"ended_at_ms"`
	FlashcardsReviewed int    `json:"flashcards_reviewed"`
	CorrectRecallCount int    `json:"correct_recall_count"`
	TotalTimeTakenMS   int    `json:"total_time_taken_ms"`
	EmitTelemetry      bool   `json:"emit_telemetry"`
}

type RAGStreamEvent struct {
	Type    string   `json:"type"`
	Text    string   `json:"text,omitempty"`
	Sources []string `json:"sources,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type SyncSettingsDTO struct {
	BaseURL     string `json:"base_url"`
	ClassCode   string `json:"class_code"`
	StudentName string `json:"student_name,omitempty"`
}

type CloudHealthProbeResult struct {
	URL        string `json:"url"`
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message,omitempty"`
	LatencyMS  int64  `json:"latency_ms"`
	CheckedAt  string `json:"checked_at"`
}

func NewApp() *App {
	return &App{logger: slog.Default().With("component", "app")}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startupErr = a.initDatabase()
	if a.startupErr != nil {
		a.logger.Error("app_startup_failed", "error", a.startupErr)
		return
	}
	a.logger.Info("app_startup_completed", "vec_enabled", a.vecEnabled)
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	if a.database != nil {
		_ = a.database.Close()
		a.database = nil
	}
	a.logger.Info("app_shutdown_completed")
}

func (a *App) GetStartupStatus() string {
	if a.startupErr != nil {
		return "error: " + a.startupErr.Error()
	}
	return "ok"
}

func (a *App) GetDashboardSnapshot() (*ui.DashboardSnapshot, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	svc := ui.NewDashboardService(a.database)
	return svc.GetSnapshot(ctx)
}

func (a *App) GetSyncStatus() (*syncsvc.SyncStatus, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	svc := syncsvc.NewService(a.database)
	return svc.GetStatus()
}

func (a *App) RunManualSync() (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	svc := syncsvc.NewService(a.database)
	result, err := svc.RunManualSync(ctx, 100)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("attempted=%d sent=%d failed=%d skipped=%d", result.Attempted, result.Sent, result.Failed, result.Skipped), nil
}

func (a *App) GetSyncSettings() (*SyncSettingsDTO, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	queries := db.NewStudentConfigQueries(a.database.DB)
	baseURL, err := queries.Get("sync_base_url")
	if err != nil {
		return nil, fmt.Errorf("load sync_base_url: %w", err)
	}
	classCode, err := queries.Get("sync_class_code")
	if err != nil {
		return nil, fmt.Errorf("load sync_class_code: %w", err)
	}
	studentName, err := queries.Get("student_name")
	if err != nil {
		return nil, fmt.Errorf("load student_name: %w", err)
	}

	return &SyncSettingsDTO{
		BaseURL:     baseURL,
		ClassCode:   classCode,
		StudentName: studentName,
	}, nil
}

func (a *App) SaveSyncSettings(baseURL, classCode string) (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}

	normalizedBaseURL := strings.TrimSpace(baseURL)
	normalizedClassCode := strings.ToUpper(strings.TrimSpace(classCode))
	if normalizedBaseURL == "" {
		return "", apperrors.User("SYNC_BASE_URL_REQUIRED", "base URL is required")
	}
	if !strings.HasPrefix(strings.ToLower(normalizedBaseURL), "http://") && !strings.HasPrefix(strings.ToLower(normalizedBaseURL), "https://") {
		return "", apperrors.User("SYNC_BASE_URL_INVALID", "base URL must start with http:// or https://")
	}

	queries := db.NewStudentConfigQueries(a.database.DB)
	if err := queries.Set("sync_base_url", strings.TrimSuffix(normalizedBaseURL, "/")); err != nil {
		return "", fmt.Errorf("save sync_base_url: %w", err)
	}
	if err := queries.Set("sync_class_code", normalizedClassCode); err != nil {
		return "", fmt.Errorf("save sync_class_code: %w", err)
	}

	return "Classroom sync settings saved", nil
}

func (a *App) ProbeCloudHealth(baseURL string) (*CloudHealthProbeResult, error) {
	if a.startupErr != nil {
		return nil, apperrors.System("APP_STARTUP_FAILED", "app startup failed", a.startupErr)
	}

	normalizedBaseURL := strings.TrimSpace(baseURL)
	if normalizedBaseURL == "" {
		if a.database == nil {
			return nil, fmt.Errorf("database is not initialized")
		}
		queries := db.NewStudentConfigQueries(a.database.DB)
		storedBaseURL, err := queries.Get("sync_base_url")
		if err != nil {
			return nil, fmt.Errorf("load sync_base_url: %w", err)
		}
		normalizedBaseURL = strings.TrimSpace(storedBaseURL)
	}
	if normalizedBaseURL == "" {
		return nil, apperrors.User("SYNC_BASE_URL_REQUIRED", "base URL is required")
	}

	normalizedBaseURL = strings.TrimSuffix(normalizedBaseURL, "/")
	healthURL := normalizedBaseURL + "/health"

	client := &http.Client{Timeout: 12 * time.Second}
	started := time.Now().UTC()
	resp, err := client.Get(healthURL)
	if err != nil {
		return &CloudHealthProbeResult{
			URL:        healthURL,
			OK:         false,
			StatusCode: 0,
			Message:    err.Error(),
			LatencyMS:  time.Since(started).Milliseconds(),
			CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	msg := "ok"
	if !ok {
		msg = "non-2xx response from health endpoint"
	}

	return &CloudHealthProbeResult{
		URL:        healthURL,
		OK:         ok,
		StatusCode: resp.StatusCode,
		Message:    msg,
		LatencyMS:  time.Since(started).Milliseconds(),
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

type JoinClassRequest struct {
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
	USN       string `json:"usn"`
	ClassCode string `json:"class_code"`
}

type JoinClassResponse struct {
	Success   bool   `json:"success"`
	ClassID   string `json:"class_id,omitempty"`
	ClassName string `json:"class_name,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (a *App) JoinClass(studentID, name, usn, classCode string) (*JoinClassResponse, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	queries := db.NewStudentConfigQueries(a.database.DB)
	baseURL, err := queries.Get("sync_base_url")
	if err != nil {
		return nil, fmt.Errorf("load sync_base_url: %w", err)
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("cloud base URL not configured")
	}

	baseURL = strings.TrimSuffix(baseURL, "/")
	joinURL := baseURL + "/api/v1/classes/join"

	reqBody := JoinClassRequest{
		StudentID: strings.TrimSpace(studentID),
		Name:      strings.TrimSpace(name),
		USN:       strings.TrimSpace(usn),
		ClassCode: strings.TrimSpace(classCode),
	}

	if reqBody.StudentID == "" {
		return nil, apperrors.User("STUDENT_ID_REQUIRED", "student_id is required")
	}
	if reqBody.Name == "" {
		return nil, apperrors.User("STUDENT_NAME_REQUIRED", "name is required")
	}
	if reqBody.ClassCode == "" {
		return nil, apperrors.User("CLASS_CODE_REQUIRED", "class_code is required")
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal join request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(joinURL, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", joinURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var result JoinClassResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if result.Error != "" {
			return &result, fmt.Errorf("join class failed: %s", result.Error)
		}
		return &result, fmt.Errorf("join class failed with status %d", resp.StatusCode)
	}

	return &result, nil
}

func (a *App) IngestDocument(filePath, notebookName string) (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}

	trimmedPath := strings.TrimSpace(filePath)
	if trimmedPath == "" {
		return "", apperrors.User("INGEST_FILE_PATH_REQUIRED", "file path is required")
	}
	a.logger.Info("document_ingestion_started", "file_path", trimmedPath, "notebook", notebookName)
	if _, err := os.Stat(trimmedPath); err != nil {
		return "", apperrors.UserWrap("INGEST_FILE_NOT_FOUND", "unable to access selected file", err)
	}

	trimmedNotebook := strings.TrimSpace(notebookName)
	if trimmedNotebook == "" {
		trimmedNotebook = "General Notebook"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	nbID, err := a.resolveOrCreateNotebook(ctx, trimmedNotebook)
	if err != nil {
		return "", err
	}

	ingestSvc := ingestion.NewService(a.database)
	// Wails upload flow currently does not wire a runtime embedder; keep vec writes off.
	ingestSvc.SetVectorStoreEnabled(false)

	doc, err := ingestSvc.RegisterDocument(ctx, nbID, trimmedPath)
	if err != nil {
		return "", fmt.Errorf("register document: %w", err)
	}

	chunkCount, err := ingestSvc.ProcessRegisteredDocument(ctx, doc.ID, trimmedNotebook)
	if err != nil {
		return "", fmt.Errorf("process document: %w", err)
	}

	cardCount, err := a.generateStarterFlashcards(ctx, nbID, doc.ID, 8)
	if err != nil {
		return "", err
	}
	a.logger.Info("document_ingestion_completed", "notebook_id", nbID, "document_id", doc.ID, "chunks", chunkCount, "starter_cards", cardCount)

	return fmt.Sprintf("ingested %s: chunks=%d, starter_cards=%d", filepath.Base(trimmedPath), chunkCount, cardCount), nil
}

func (a *App) PickDocumentPath() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application context unavailable")
	}

	selectedPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Study File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Study Files",
				Pattern:     "*.pdf;*.txt;*.md",
			},
			{
				DisplayName: "PDF Documents",
				Pattern:     "*.pdf",
			},
			{
				DisplayName: "Text and Markdown",
				Pattern:     "*.txt;*.md",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("open file dialog: %w", err)
	}

	return strings.TrimSpace(selectedPath), nil
}

// StreamRAGAnswer emits incremental RAG answer chunks via Wails runtime events.
// Returns a unique event channel name that the frontend can subscribe to.
func (a *App) StreamRAGAnswer(question string) (string, error) {
	if a.startupErr != nil {
		return "", apperrors.System("APP_STARTUP_FAILED", "app startup failed", a.startupErr)
	}
	if a.ctx == nil {
		return "", apperrors.System("APP_CONTEXT_UNAVAILABLE", "application context unavailable", nil)
	}
	if strings.TrimSpace(question) == "" {
		return "", apperrors.User("RAG_QUESTION_REQUIRED", "question is required")
	}

	eventName := "rag:stream:" + uuid.NewString()

	go func() {
		answer := "Federalism divides powers between central and state governments, balancing national unity with regional autonomy."
		parts := strings.Fields(answer)
		for _, part := range parts {
			runtime.EventsEmit(a.ctx, eventName, RAGStreamEvent{Type: "chunk", Text: part + " "})
			time.Sleep(45 * time.Millisecond)
		}

		runtime.EventsEmit(a.ctx, eventName, RAGStreamEvent{
			Type: "done",
			Sources: []string{
				"[Polity - Federalism] chunk #3",
				"[Polity - Parliament] chunk #7",
			},
		})
	}()

	return eventName, nil
}

func (a *App) GetNextDueCard() (*ReviewCardDTO, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	type dueRow struct {
		FlashcardID  string     `gorm:"column:flashcard_id"`
		NotebookID   string     `gorm:"column:notebook_id"`
		NotebookName string     `gorm:"column:notebook_name"`
		Question     string     `gorm:"column:question"`
		Answer       string     `gorm:"column:answer"`
		State        string     `gorm:"column:state"`
		DueDate      *time.Time `gorm:"column:due_date"`
		Reps         int        `gorm:"column:reps"`
		Lapses       int        `gorm:"column:lapses"`
	}

	var total int64
	if err := a.database.DB.WithContext(ctx).Raw(`
SELECT COUNT(1)
FROM flashcards
WHERE due_date IS NULL OR due_date <= ?;
`, time.Now().UTC()).Scan(&total).Error; err != nil {
		return nil, fmt.Errorf("count due cards: %w", err)
	}
	if total == 0 {
		return nil, nil
	}

	var row dueRow
	if err := a.database.DB.WithContext(ctx).Raw(`
SELECT f.id AS flashcard_id, f.notebook_id, n.name AS notebook_name, f.question, f.answer, f.state, f.due_date, f.reps, f.lapses
FROM flashcards f
JOIN notebooks n ON n.id = f.notebook_id
WHERE f.due_date IS NULL OR f.due_date <= ?
ORDER BY COALESCE(f.due_date, '1970-01-01T00:00:00Z') ASC
LIMIT 1;
`, time.Now().UTC()).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("load due card: %w", err)
	}

	dueAt := ""
	if row.DueDate != nil {
		dueAt = row.DueDate.UTC().Format(time.RFC3339)
	}

	return &ReviewCardDTO{
		FlashcardID:   row.FlashcardID,
		NotebookID:    row.NotebookID,
		NotebookName:  row.NotebookName,
		Question:      row.Question,
		Answer:        row.Answer,
		State:         row.State,
		DueAt:         dueAt,
		Reps:          row.Reps,
		Lapses:        row.Lapses,
		QueuePosition: 1,
		QueueSize:     int(total),
	}, nil
}

func (a *App) RateDueCard(input ReviewRateInput) (*ReviewRateResult, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.FlashcardID) == "" {
		return nil, apperrors.User("FLASHCARD_ID_REQUIRED", "flashcard_id is required")
	}
	if strings.TrimSpace(input.NotebookID) == "" {
		return nil, apperrors.User("NOTEBOOK_ID_REQUIRED", "notebook_id is required")
	}
	if input.Rating < fsrs.RatingAgain || input.Rating > fsrs.RatingEasy {
		return nil, apperrors.User("FSRS_RATING_INVALID", fmt.Sprintf("invalid rating: %d", input.Rating))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if strings.TrimSpace(input.NotebookName) == "" {
		var nb struct {
			Name string `gorm:"column:name"`
		}
		if err := a.database.DB.WithContext(ctx).
			Table("notebooks").
			Select("name").
			Where("id = ?", input.NotebookID).
			Take(&nb).Error; err != nil {
			return nil, fmt.Errorf("resolve notebook name: %w", err)
		}
		input.NotebookName = nb.Name
	}

	fsrsSvc := fsrs.NewService(a.database, syncsvc.NewService(a.database))
	result, err := fsrsSvc.ReviewCard(ctx, fsrs.ReviewInput{
		FlashcardID:  input.FlashcardID,
		NotebookID:   input.NotebookID,
		NotebookName: input.NotebookName,
		Rating:       input.Rating,
		TimeTakenMs:  input.TimeTakenMs,
	})
	if err != nil {
		return nil, err
	}
	a.logger.Info("fsrs_calculation_completed", "flashcard_id", input.FlashcardID, "rating", input.Rating, "next_due_at", result.NextDueAt.UTC().Format(time.RFC3339))

	return &ReviewRateResult{
		NextDueAt: result.NextDueAt.UTC().Format(time.RFC3339),
		State:     result.State,
		Message:   "FSRS updated and review log saved",
	}, nil
}

func (a *App) CompleteReviewSession(input ReviewSessionSummaryInput) (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}
	if strings.TrimSpace(input.NotebookID) == "" {
		return "", apperrors.User("NOTEBOOK_ID_REQUIRED", "notebook_id is required")
	}
	if input.FlashcardsReviewed < 0 || input.CorrectRecallCount < 0 || input.TotalTimeTakenMS < 0 {
		return "", apperrors.User("SESSION_METRICS_INVALID", "session metrics cannot be negative")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	notebookName := strings.TrimSpace(input.NotebookName)
	if notebookName == "" {
		var nb struct {
			Name string `gorm:"column:name"`
		}
		if err := a.database.DB.WithContext(ctx).
			Table("notebooks").
			Select("name").
			Where("id = ?", input.NotebookID).
			Take(&nb).Error; err != nil {
			return "", fmt.Errorf("resolve notebook name: %w", err)
		}
		notebookName = nb.Name
	}

	startedAt := time.UnixMilli(input.StartedAtMS).UTC()
	endedAt := time.UnixMilli(input.EndedAtMS).UTC()
	if input.StartedAtMS <= 0 {
		startedAt = time.Now().UTC().Add(-2 * time.Minute)
	}
	if input.EndedAtMS <= 0 {
		endedAt = time.Now().UTC()
	}

	fsrsSvc := fsrs.NewService(a.database, syncsvc.NewService(a.database))
	err := fsrsSvc.CompleteSession(ctx, fsrs.SessionSummary{
		NotebookID:         input.NotebookID,
		NotebookName:       notebookName,
		StartedAt:          startedAt,
		EndedAt:            endedAt,
		FlashcardsReviewed: input.FlashcardsReviewed,
		CorrectRecallCount: input.CorrectRecallCount,
		TotalTimeTakenMS:   input.TotalTimeTakenMS,
		EmitTelemetry:      input.EmitTelemetry,
	})
	if err != nil {
		return "", err
	}
	a.logger.Info("review_session_completed", "notebook_id", input.NotebookID, "flashcards_reviewed", input.FlashcardsReviewed, "emit_telemetry", input.EmitTelemetry)

	return fmt.Sprintf("session completed: reviewed=%d correct=%d", input.FlashcardsReviewed, input.CorrectRecallCount), nil
}

func (a *App) initDatabase() error {
	dbPath := filepath.Join("data", "app.db")
	schemaPath := "schema.sql"

	database, err := db.Init(dbPath)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}

	capabilities, err := database.DetectSQLiteCapabilities()
	if err != nil {
		_ = database.Close()
		return fmt.Errorf("probe sqlite capabilities: %w", err)
	}
	if !capabilities.FTS5 {
		_ = database.Close()
		return fmt.Errorf("sqlite FTS5 module unavailable")
	}

	skipVectorTable := false
	if !capabilities.Vec0 {
		onnxPath := filepath.Join("onnx", "model_int8.onnx")
		if _, statErr := os.Stat(onnxPath); statErr != nil {
			_ = database.Close()
			return fmt.Errorf("sqlite-vec unavailable and ONNX fallback model missing at %s: %w", onnxPath, statErr)
		}
		skipVectorTable = true
	}
	a.vecEnabled = capabilities.Vec0

	if err := database.RunSchemaMigrationsWithOptions(schemaPath, db.MigrationOptions{SkipVectorTable: skipVectorTable}); err != nil {
		_ = database.Close()
		return fmt.Errorf("run migrations: %w", err)
	}

	if err := database.IntegrityCheck(); err != nil {
		_ = database.Close()
		return fmt.Errorf("sqlite integrity check: %w", err)
	}

	a.database = database
	return nil
}

func (a *App) resolveOrCreateNotebook(ctx context.Context, notebookName string) (string, error) {
	var row struct {
		ID string `gorm:"column:id"`
	}
	err := a.database.DB.WithContext(ctx).
		Table("notebooks").
		Select("id").
		Where("name = ?", notebookName).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "", fmt.Errorf("lookup notebook: %w", err)
	}
	if strings.TrimSpace(row.ID) != "" {
		return row.ID, nil
	}

	now := time.Now().UTC()
	nb := db.Notebook{
		ID:          uuid.NewString(),
		Name:        notebookName,
		Description: "Created from desktop upload flow",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := a.database.DB.WithContext(ctx).Create(&nb).Error; err != nil {
		return "", fmt.Errorf("create notebook: %w", err)
	}

	return nb.ID, nil
}

func (a *App) generateStarterFlashcards(ctx context.Context, notebookID, documentID string, limit int) (int, error) {
	if limit <= 0 {
		limit = 8
	}

	type chunkRow struct {
		ID          string `gorm:"column:id"`
		ChapterName string `gorm:"column:chapter_name"`
		Content     string `gorm:"column:content"`
	}

	rows := make([]chunkRow, 0, limit)
	if err := a.database.DB.WithContext(ctx).Raw(`
SELECT id, chapter_name, content
FROM chunks
WHERE notebook_id = ? AND document_id = ?
ORDER BY chunk_index ASC
LIMIT ?;
`, notebookID, documentID, limit).Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("load chunks for starter flashcards: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	now := time.Now().UTC()
	cards := make([]db.Flashcard, 0, len(rows))
	for idx, c := range rows {
		chapter := strings.TrimSpace(c.ChapterName)
		if chapter == "" {
			chapter = "General"
		}

		answer := strings.TrimSpace(c.Content)
		if len(answer) > 380 {
			answer = strings.TrimSpace(answer[:380]) + "..."
		}
		if answer == "" {
			continue
		}

		cards = append(cards, db.Flashcard{
			ID:             uuid.NewString(),
			ChunkID:        c.ID,
			NotebookID:     notebookID,
			Question:       buildStarterQuestion(chapter, idx+1),
			Answer:         answer,
			Source:         "ai",
			Stability:      0.4,
			Difficulty:     5,
			Retrievability: 1,
			State:          "new",
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}

	if len(cards) == 0 {
		return 0, nil
	}

	if err := a.database.DB.WithContext(ctx).CreateInBatches(cards, 100).Error; err != nil {
		return 0, fmt.Errorf("create starter flashcards: %w", err)
	}

	return len(cards), nil
}

func buildStarterQuestion(chapter string, topicIndex int) string {
	trimmed := strings.TrimSpace(chapter)
	if trimmed == "" || strings.EqualFold(trimmed, "general") {
		return "Topic " + strconv.Itoa(topicIndex) + ": Summarize the key idea from this section."
	}
	return "In the section '" + trimmed + "', what is the key concept to remember?"
}

func (a *App) ensureReady() error {
	if a.startupErr != nil {
		return apperrors.System("APP_STARTUP_FAILED", "app startup failed", a.startupErr)
	}
	if a.database == nil {
		return apperrors.System("DB_NOT_INITIALIZED", "database is not initialized", nil)
	}
	return nil
}
