package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
	"github.com/yuzvak/flashsale-service/internal/config"
)

func RunMigrations(cfg config.DatabaseConfig) error {
	log.Printf("Starting migrations with config: host=%s, port=%d, user=%s, dbname=%s, migrations_path=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.DBName, cfg.MigrationsPath)

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)
	log.Printf("Connection string built (password hidden): host=%s port=%d user=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.DBName, cfg.SSLMode)

	db, dbErr := sql.Open("postgres", connStr)
	if dbErr != nil {
		log.Printf("Failed to open database connection: %v", dbErr)
		return fmt.Errorf("failed to open database connection: %v", dbErr)
	}
	defer db.Close()
	log.Printf("Database connection opened successfully")

	if err := db.Ping(); err != nil {
		log.Printf("Failed to ping database: %v", err)
		return fmt.Errorf("failed to ping database: %v", err)
	}
	log.Printf("Database ping successful")

	log.Printf("Creating migrations table if it doesn't exist")
	_, dbErr = db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if dbErr != nil {
		log.Printf("Failed to create migrations table: %v", dbErr)
		return fmt.Errorf("failed to create migrations table: %v", dbErr)
	}
	log.Printf("Migrations table created or already exists")

	log.Printf("Getting list of applied migrations")
	rows, queryErr := db.Query("SELECT name FROM migrations")
	if queryErr != nil {
		log.Printf("Failed to query migrations table: %v", queryErr)
		return fmt.Errorf("failed to query migrations table: %v", queryErr)
	}
	defer rows.Close()

	appliedMigrations := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		appliedMigrations[name] = true
	}

	log.Printf("Reading migrations from directory: %s", cfg.MigrationsPath)
	files, err := os.ReadDir(cfg.MigrationsPath)
	if err != nil {
		log.Printf("Failed to read migrations directory %s: %v", cfg.MigrationsPath, err)
		return fmt.Errorf("failed to read migrations directory %s: %v", cfg.MigrationsPath, err)
	}
	log.Printf("Found %d files in migrations directory", len(files))

	var migrations []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".up.sql") {
			log.Printf("Found migration file: %s", file.Name())
			migrations = append(migrations, file.Name())
		}
	}
	sort.Strings(migrations)
	log.Printf("Found %d migration files to process", len(migrations))

	log.Printf("Starting to apply migrations")
	for _, migration := range migrations {
		if appliedMigrations[migration] {
			log.Printf("Migration %s already applied, skipping", migration)
			continue
		}

		log.Printf("Applying migration: %s", migration)
		filePath := filepath.Join(cfg.MigrationsPath, migration)
		log.Printf("Reading migration file: %s", filePath)
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("Failed to read migration file %s: %v", filePath, err)
			return fmt.Errorf("failed to read migration file %s: %v", filePath, err)
		}
		log.Printf("Migration file read successfully, content length: %d bytes", len(content))

		log.Printf("Beginning transaction for migration %s", migration)
		tx, err := db.Begin()
		if err != nil {
			log.Printf("Failed to begin transaction: %v", err)
			return fmt.Errorf("failed to begin transaction: %v", err)
		}

		log.Printf("Executing migration SQL for %s", migration)
		_, err = tx.Exec(string(content))
		if err != nil {
			log.Printf("Failed to execute migration %s: %v", migration, err)
			tx.Rollback()
			return fmt.Errorf("error executing migration %s: %v", migration, err)
		}
		log.Printf("Migration SQL executed successfully for %s", migration)

		log.Printf("Recording migration %s in migrations table", migration)
		_, err = tx.Exec("INSERT INTO migrations (name) VALUES ($1)", migration)
		if err != nil {
			log.Printf("Failed to record migration %s: %v", migration, err)
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %v", migration, err)
		}

		log.Printf("Committing transaction for migration %s", migration)
		if err := tx.Commit(); err != nil {
			log.Printf("Failed to commit transaction for migration %s: %v", migration, err)
			return fmt.Errorf("failed to commit transaction for migration %s: %v", migration, err)
		}

		log.Printf("Successfully applied migration: %s", migration)
		fmt.Printf("Applied migration: %s\n", migration)
	}

	log.Printf("All migrations completed successfully")
	return nil
}
