package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database represents the SQLite connection pool
type Database struct {
	DB *gorm.DB
}

// SQLiteCapabilities captures optional module availability in current runtime.
type SQLiteCapabilities struct {
	FTS5 bool
	Vec0 bool
}

// MigrationOptions configures runtime schema behavior.
type MigrationOptions struct {
	SkipVectorTable bool
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
		&Topic{},
		&DailyTask{},
		&Flashcard{},
		&ReviewLog{},
		&QuizSession{},
		&StudySession{},
		&EducationalTelemetry{},
		&AIDiagnosticTelemetry{},
		&SyncQueueItem{},
		&StudentConfig{},
	)
}

// RunSchemaMigrations executes raw SQL migrations (for FTS5 and sqlite-vec setup)
func (d *Database) RunSchemaMigrations(schemaPath string) error {
	return d.RunSchemaMigrationsWithOptions(schemaPath, MigrationOptions{})
}

// RunSchemaMigrationsWithOptions executes raw SQL migrations with runtime options.
func (d *Database) RunSchemaMigrationsWithOptions(schemaPath string, opts MigrationOptions) error {
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	schemaSQL := string(schema)
	if opts.SkipVectorTable {
		schemaSQL = stripVectorTableDDL(schemaSQL)
	}

	if err := d.DB.Exec(schemaSQL).Error; err != nil {
		return fmt.Errorf("failed to run schema migrations: %w", err)
	}

	return nil
}

// DetectSQLiteCapabilities probes whether required optional SQLite modules are available.
func (d *Database) DetectSQLiteCapabilities() (SQLiteCapabilities, error) {
	fts5OK, err := d.checkVirtualModule("fts5", "CREATE VIRTUAL TABLE temp.__fts5_probe USING fts5(content)")
	if err != nil {
		return SQLiteCapabilities{}, err
	}

	vec0OK, err := d.checkVirtualModule("vec0", "CREATE VIRTUAL TABLE temp.__vec0_probe USING vec0(embedding float[3])")
	if err != nil {
		return SQLiteCapabilities{}, err
	}

	return SQLiteCapabilities{FTS5: fts5OK, Vec0: vec0OK}, nil
}

func (d *Database) checkVirtualModule(moduleName, createSQL string) (bool, error) {
	// Use a silent logger for the probe to avoid misleading error logs when the module is expectedly missing
	session := d.DB.Session(&gorm.Session{Logger: d.DB.Logger.LogMode(logger.Silent)})
	if err := session.Exec(createSQL).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such module: "+strings.ToLower(moduleName)) {
			return false, nil
		}
		return false, fmt.Errorf("failed probing sqlite module %s: %w", moduleName, err)
	}

	dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS temp.__%s_probe", moduleName)
	if err := session.Exec(dropSQL).Error; err != nil {
		return false, fmt.Errorf("failed cleanup sqlite module probe %s: %w", moduleName, err)
	}

	return true, nil
}

// IntegrityCheck verifies SQLite internal consistency.
func (d *Database) IntegrityCheck() error {
	type integrityRow struct {
		Result string `gorm:"column:integrity_check"`
	}

	var row integrityRow
	if err := d.DB.Raw("PRAGMA integrity_check").Scan(&row).Error; err != nil {
		return fmt.Errorf("run sqlite integrity check: %w", err)
	}

	if strings.ToLower(strings.TrimSpace(row.Result)) != "ok" {
		return fmt.Errorf("sqlite integrity check failed: %s", row.Result)
	}

	return nil
}

func stripVectorTableDDL(schemaSQL string) string {
	lines := strings.Split(schemaSQL, "\n")
	out := make([]string, 0, len(lines))
	skip := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if !skip && strings.HasPrefix(upper, "CREATE VIRTUAL TABLE IF NOT EXISTS EMBEDDINGS USING VEC0(") {
			skip = true
			continue
		}

		if skip {
			if strings.Contains(trimmed, ");") {
				skip = false
			}
			continue
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
