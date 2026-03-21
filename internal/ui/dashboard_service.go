package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-tutor-local/internal/db"
)

// DashboardSnapshot is the frontend-ready shape for home dashboard state.
type DashboardSnapshot struct {
	DueToday        int64                `json:"due_today"`
	StudyStreak     int                  `json:"study_streak_days"`
	ActiveNotebooks int                  `json:"active_notebooks"`
	PendingSync     int64                `json:"pending_sync"`
	Notebooks       []NotebookSummary    `json:"notebooks"`
	Ingestion       []IngestionStatusRow `json:"ingestion"`
	SyncStatusText  string               `json:"sync_status_text"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

type NotebookSummary struct {
	NotebookID string `json:"notebook_id"`
	Name       string `json:"name"`
	Documents  int64  `json:"documents"`
}

type IngestionStatusRow struct {
	NotebookName string `json:"notebook_name"`
	Filename     string `json:"filename"`
	Status       string `json:"status"`
	ProgressPct  int    `json:"progress_pct"`
}

// DashboardService provides dashboard-focused read APIs for frontend bindings.
type DashboardService struct {
	database *db.Database
}

func NewDashboardService(database *db.Database) *DashboardService {
	return &DashboardService{database: database}
}

// GetSnapshot returns a consolidated dashboard summary for home screen rendering.
func (s *DashboardService) GetSnapshot(ctx context.Context) (*DashboardSnapshot, error) {
	now := time.Now().UTC()
	snapshot := &DashboardSnapshot{GeneratedAt: now}

	if err := s.database.DB.WithContext(ctx).Raw(`
SELECT COUNT(1)
FROM flashcards
WHERE due_date IS NULL OR due_date <= ?;
`, now).Scan(&snapshot.DueToday).Error; err != nil {
		return nil, fmt.Errorf("count due cards: %w", err)
	}

	if err := s.database.DB.WithContext(ctx).Raw(`
SELECT COUNT(1)
FROM notebooks;
`).Scan(&snapshot.ActiveNotebooks).Error; err != nil {
		return nil, fmt.Errorf("count notebooks: %w", err)
	}

	if err := s.database.DB.WithContext(ctx).Raw(`
SELECT COUNT(1)
FROM sync_queue
WHERE status = 'pending';
`).Scan(&snapshot.PendingSync).Error; err != nil {
		return nil, fmt.Errorf("count pending sync rows: %w", err)
	}

	notebooks := make([]NotebookSummary, 0)
	if err := s.database.DB.WithContext(ctx).Raw(`
SELECT n.id AS notebook_id, n.name, COUNT(d.id) AS documents
FROM notebooks n
LEFT JOIN documents d ON d.notebook_id = n.id
GROUP BY n.id, n.name
ORDER BY n.updated_at DESC
LIMIT 6;
`).Scan(&notebooks).Error; err != nil {
		return nil, fmt.Errorf("list notebook summary: %w", err)
	}
	snapshot.Notebooks = notebooks

	ingestion := make([]IngestionStatusRow, 0)
	if err := s.database.DB.WithContext(ctx).Raw(`
SELECT n.name AS notebook_name, d.filename, d.status
FROM documents d
JOIN notebooks n ON n.id = d.notebook_id
ORDER BY d.created_at DESC
LIMIT 8;
`).Scan(&ingestion).Error; err != nil {
		return nil, fmt.Errorf("list ingestion status: %w", err)
	}
	for i := range ingestion {
		ingestion[i].ProgressPct = mapStatusProgress(ingestion[i].Status)
	}
	snapshot.Ingestion = ingestion

	streak, err := s.computeStudyStreak(ctx, now)
	if err != nil {
		return nil, err
	}
	snapshot.StudyStreak = streak
	snapshot.SyncStatusText = s.makeSyncStatusText(snapshot.PendingSync)

	return snapshot, nil
}

func (s *DashboardService) computeStudyStreak(ctx context.Context, now time.Time) (int, error) {
	rows := make([]struct {
		StudyDay string `gorm:"column:study_day"`
	}, 0)

	if err := s.database.DB.WithContext(ctx).Raw(`
SELECT DISTINCT DATE(ended_at) AS study_day
FROM study_sessions
ORDER BY study_day DESC
LIMIT 60;
`).Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("load study days: %w", err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	daySet := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		day := strings.TrimSpace(r.StudyDay)
		if day != "" {
			daySet[day] = struct{}{}
		}
	}

	streak := 0
	for d := now; ; d = d.AddDate(0, 0, -1) {
		key := d.Format("2006-01-02")
		if _, ok := daySet[key]; !ok {
			break
		}
		streak++
	}

	return streak, nil
}

func mapStatusProgress(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready":
		return 100
	case "processing":
		return 55
	case "pending":
		return 10
	case "error":
		return 0
	default:
		return 0
	}
}

func (s *DashboardService) makeSyncStatusText(pending int64) string {
	if pending == 0 {
		return "Sync queue is clear. Periodic sync healthy."
	}
	if pending == 1 {
		return "1 event pending in sync queue. Retry worker active."
	}
	return fmt.Sprintf("%d events pending in sync queue. Retry worker active.", pending)
}
