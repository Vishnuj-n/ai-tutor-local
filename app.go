package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-tutor-local/internal/db"
	"ai-tutor-local/internal/ui"
)

// App is the Wails application state.
type App struct {
	ctx        context.Context
	database   *db.Database
	startupErr error
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startupErr = a.initDatabase()
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	if a.database != nil {
		_ = a.database.Close()
		a.database = nil
	}
}

func (a *App) GetStartupStatus() string {
	if a.startupErr != nil {
		return "error: " + a.startupErr.Error()
	}
	return "ok"
}

func (a *App) GetDashboardSnapshot() (*ui.DashboardSnapshot, error) {
	if a.startupErr != nil {
		return nil, fmt.Errorf("app startup failed: %w", a.startupErr)
	}
	if a.database == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	svc := ui.NewDashboardService(a.database)
	return svc.GetSnapshot(ctx)
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

	if err := database.RunSchemaMigrationsWithOptions(schemaPath, db.MigrationOptions{SkipVectorTable: skipVectorTable}); err != nil {
		_ = database.Close()
		return fmt.Errorf("run migrations: %w", err)
	}

	a.database = database
	return nil
}

func envBool(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes"
}
