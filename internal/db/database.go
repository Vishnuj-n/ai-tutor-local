package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database represents the SQLite connection pool
type Database struct {
	DB *gorm.DB
}

var dbInstance *Database

// Init initializes the SQLite database connection
// dbPath should be the full path to the SQLite file (e.g., ~/.config/ai-tutor-local/app.db)
func Init(dbPath string) (*Database, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Open connection with proper SQLite configuration
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Enable foreign keys
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	dbInstance = &Database{DB: db}
	return dbInstance, nil
}

// GetDB returns the singleton database instance
func GetDB() *gorm.DB {
	if dbInstance == nil {
		log.Fatal("Database not initialized. Call db.Init() first")
	}
	return dbInstance.DB
}

// Close closes the database connection
func (d *Database) Close() error {
	db, err := d.DB.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// Migrate runs all schema migration functions
func (d *Database) Migrate() error {
	return d.DB.AutoMigrate(
		&Notebook{},
		&Document{},
		&Chunk{},
		&Flashcard{},
		&ReviewLog{},
		&QuizSession{},
		&StudySession{},
		&SyncQueueItem{},
		&StudentConfig{},
	)
}

// RunSchemaMigrations executes raw SQL migrations (for FTS5 and sqlite-vec setup)
func (d *Database) RunSchemaMigrations(schemaPath string) error {
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	if err := d.DB.Exec(string(schema)).Error; err != nil {
		return fmt.Errorf("failed to run schema migrations: %w", err)
	}

	return nil
}
