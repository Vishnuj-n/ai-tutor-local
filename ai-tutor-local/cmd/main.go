package main

import (
	"fmt"
	"log"
	"path/filepath"

	"ai-tutor-local/internal/db"
)

func main() {
	dbPath := filepath.Join("data", "app.db")
	schemaPath := "schema.sql"

	database, err := db.Init(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			log.Printf("database close error: %v", closeErr)
		}
	}()

	if err := database.RunSchemaMigrations(schemaPath); err != nil {
		log.Fatalf("failed to run schema migrations: %v", err)
	}

	fmt.Println("ai-tutor-local Track A foundation ready")
}
