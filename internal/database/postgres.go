package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"oms/internal/config"
)

func Connect(cfg *config.Config) (*sql.DB, error) {
	dsn := cfg.DSN()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Quick check if DB is reachable
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("PostgreSQL database ping failed: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL database")
	return db, nil
}

func InitSchema(db *sql.DB, migrationFile string) error {
	content, err := os.ReadFile(migrationFile)
	if err != nil {
		log.Printf("Migration file not found at %s, relying on pre-initialized schema", migrationFile)
		return nil
	}

	_, err = db.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to execute database initialization script: %w", err)
	}

	log.Println("Database schema & seed data successfully initialized")
	return nil
}
