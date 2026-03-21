package fsrs

import (
	"context"
	"fmt"
	"math"
	"time"

	"ai-tutor-local/internal/db"
	syncsvc "ai-tutor-local/internal/sync"

	"github.com/google/uuid"
)

// Rating options for FSRS-style review updates.
const (
	RatingAgain = 1
	RatingHard  = 2
	RatingGood  = 3
	RatingEasy  = 4
)

// ReviewInput captures one card-rating action.
type ReviewInput struct {
	FlashcardID   string
	NotebookID    string
	NotebookName  string
	Rating        int
	TimeTakenMs   int
	SessionID     string
	SessionStart  time.Time
	SessionEnd    time.Time
	EmitTelemetry bool
}

// ReviewResult contains updated scheduling fields after rating.
type ReviewResult struct {
	FlashcardID    string
	NextDueAt      time.Time
	State          string
	Stability      float32
	Difficulty     float32
	Retrievability float32
	Reps           int
	Lapses         int
}

// SessionSummary is persisted at review session end.
type SessionSummary struct {
	SessionID          string
	NotebookID         string
	NotebookName       string
	StartedAt          time.Time
	EndedAt            time.Time
	FlashcardsReviewed int
	CorrectRecallCount int
	TotalTimeTakenMS   int
	EmitTelemetry      bool
}

// Service handles Sprint 3 review workflow orchestration.
type Service struct {
	database *db.Database
	sync     *syncsvc.Service
}

func NewService(database *db.Database, syncService *syncsvc.Service) *Service {
	return &Service{database: database, sync: syncService}
}

func (s *Service) ReviewCard(ctx context.Context, in ReviewInput) (*ReviewResult, error) {
	if in.Rating < RatingAgain || in.Rating > RatingEasy {
		return nil, fmt.Errorf("invalid rating: %d", in.Rating)
	}
	if in.TimeTakenMs < 0 {
		return nil, fmt.Errorf("time_taken_ms cannot be negative")
	}

	var card struct {
		ID             string  `gorm:"column:id"`
		NotebookID     string  `gorm:"column:notebook_id"`
		Stability      float32 `gorm:"column:stability"`
		Difficulty     float32 `gorm:"column:difficulty"`
		Retrievability float32 `gorm:"column:retrievability"`
		Reps           int     `gorm:"column:reps"`
		Lapses         int     `gorm:"column:lapses"`
		State          string  `gorm:"column:state"`
	}

	if err := s.database.DB.WithContext(ctx).
		Table("flashcards").
		Select("id", "notebook_id", "stability", "difficulty", "retrievability", "reps", "lapses", "state").
		Where("id = ?", in.FlashcardID).
		Take(&card).Error; err != nil {
		return nil, fmt.Errorf("load flashcard: %w", err)
	}

	now := time.Now().UTC()
	next := applyFSRS(card.Stability, card.Difficulty, card.Retrievability, card.Reps, card.Lapses, in.Rating)
	nextDue := now.Add(next.interval)

	tx := s.database.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table("flashcards").Where("id = ?", in.FlashcardID).Updates(map[string]interface{}{
		"stability":      next.stability,
		"difficulty":     next.difficulty,
		"retrievability": next.retrievability,
		"due_date":       nextDue,
		"reps":           next.reps,
		"lapses":         next.lapses,
		"state":          next.state,
		"last_review":    now,
		"updated_at":     now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("update flashcard schedule: %w", err)
	}

	if err := tx.Create(&db.ReviewLog{
		ID:          uuid.NewString(),
		FlashcardID: in.FlashcardID,
		Rating:      in.Rating,
		ReviewedAt:  now,
		TimeTakenMs: in.TimeTakenMs,
	}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("insert review log: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("commit review transaction: %w", err)
	}

	return &ReviewResult{
		FlashcardID:    in.FlashcardID,
		NextDueAt:      nextDue,
		State:          next.state,
		Stability:      next.stability,
		Difficulty:     next.difficulty,
		Retrievability: next.retrievability,
		Reps:           next.reps,
		Lapses:         next.lapses,
	}, nil
}

func (s *Service) CompleteSession(ctx context.Context, in SessionSummary) error {
	if in.FlashcardsReviewed < 0 || in.CorrectRecallCount < 0 || in.TotalTimeTakenMS < 0 {
		return fmt.Errorf("session metrics cannot be negative")
	}
	if in.EndedAt.Before(in.StartedAt) {
		return fmt.Errorf("session end cannot be before session start")
	}

	durationSec := int(in.EndedAt.Sub(in.StartedAt).Seconds())
	if durationSec < 0 {
		durationSec = 0
	}

	var accuracy *float32
	if in.FlashcardsReviewed > 0 {
		acc := float32(in.CorrectRecallCount) * 100 / float32(in.FlashcardsReviewed)
		accuracy = &acc
	}

	sessionID := in.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	if err := s.database.DB.WithContext(ctx).Create(&db.StudySession{
		ID:                  sessionID,
		NotebookID:          in.NotebookID,
		ActivityType:        "flashcard",
		TimeSpentSeconds:    durationSec,
		FlashcardsCompleted: in.FlashcardsReviewed,
		AccuracyPct:         accuracy,
		StartedAt:           in.StartedAt,
		EndedAt:             in.EndedAt,
		Synced:              0,
	}).Error; err != nil {
		return fmt.Errorf("insert study session: %w", err)
	}

	if !in.EmitTelemetry || s.sync == nil {
		return nil
	}

	return s.sync.Enqueue(syncsvc.Event{
		EventID:             uuid.NewString(),
		EventType:           "flashcard_session_completed",
		NotebookID:          in.NotebookID,
		NotebookName:        in.NotebookName,
		ActivityType:        "flashcard",
		TimeSpentSeconds:    durationSec,
		FlashcardsCompleted: in.FlashcardsReviewed,
		AccuracyPct:         accuracy,
		OccurredAt:          in.EndedAt.UTC(),
	})
}

type scheduleResult struct {
	state          string
	stability      float32
	difficulty     float32
	retrievability float32
	reps           int
	lapses         int
	interval       time.Duration
}

func applyFSRS(stability, difficulty, retrievability float32, reps, lapses, rating int) scheduleResult {
	if stability <= 0 {
		stability = 0.4
	}
	if difficulty <= 0 {
		difficulty = 5
	}
	if retrievability <= 0 {
		retrievability = 1
	}

	reps++
	state := "review"
	interval := 24 * time.Hour

	switch rating {
	case RatingAgain:
		lapses++
		state = "relearning"
		stability = maxFloat32(0.2, stability*0.6+0.1)
		difficulty = clampFloat32(difficulty+0.4, 1, 10)
		retrievability = clampFloat32(retrievability*0.45, 0.05, 1)
		interval = 10 * time.Minute
	case RatingHard:
		state = "learning"
		stability = maxFloat32(0.3, stability*1.1+0.2)
		difficulty = clampFloat32(difficulty+0.1, 1, 10)
		retrievability = clampFloat32(retrievability*0.9, 0.05, 1)
		interval = 24 * time.Hour
	case RatingGood:
		stability = maxFloat32(0.4, stability*1.3+0.5)
		difficulty = clampFloat32(difficulty-0.1, 1, 10)
		retrievability = clampFloat32(retrievability*0.98+0.01, 0.05, 1)
		intervalDays := 3 + reps/2
		interval = time.Duration(intervalDays) * 24 * time.Hour
	case RatingEasy:
		stability = maxFloat32(0.5, stability*1.6+0.8)
		difficulty = clampFloat32(difficulty-0.2, 1, 10)
		retrievability = clampFloat32(retrievability*1.02, 0.05, 1)
		intervalDays := 7 + reps
		interval = time.Duration(intervalDays) * 24 * time.Hour
	}

	if state == "review" && reps == 1 {
		state = "learning"
	}

	// Smooth retrievability against very long intervals.
	stabilityDays := math.Max(float64(stability), 0.1)
	retr := float32(math.Exp(-float64(interval.Hours()/24) / stabilityDays))
	retrievability = clampFloat32((retrievability+retr)/2, 0.05, 1)

	return scheduleResult{
		state:          state,
		stability:      stability,
		difficulty:     difficulty,
		retrievability: retrievability,
		reps:           reps,
		lapses:         lapses,
		interval:       interval,
	}
}

func clampFloat32(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
