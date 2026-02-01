package repository

import (
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/ritik/twitter-fan-out/internal/config"
)

var db *sqlx.DB

// InitDB initializes the database connection
func InitDB(cfg *config.Config) (*sqlx.DB, error) {
	var err error
	db, err = sqlx.Connect("postgres", cfg.PostgresDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

// GetDB returns the database connection
func GetDB() *sqlx.DB {
	return db
}

// RunMigrations runs the SQL migration files
func RunMigrations(db *sqlx.DB, migrationsPath string) error {
	// Read and execute migration file
	migrationFile := migrationsPath + "/001_init.sql"
	content, err := os.ReadFile(migrationFile)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	_, err = db.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	return nil
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
