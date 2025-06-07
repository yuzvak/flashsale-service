package postgres

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/yuzvak/flashsale-service/internal/config"
)

type Connection struct {
	db *sql.DB
}

func NewConnection(cfg config.DatabaseConfig) (*Connection, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(50)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	return &Connection{db: db}, nil
}

func NewConnectionFromDB(db *sql.DB) *Connection {
	return &Connection{db: db}
}

func (c *Connection) Close() error {
	return c.db.Close()
}

func (c *Connection) GetDB() *sql.DB {
	return c.db
}

func (c *Connection) BeginTx() (*sql.Tx, error) {
	return c.db.Begin()
}
