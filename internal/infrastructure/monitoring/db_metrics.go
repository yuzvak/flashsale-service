package monitoring

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v4/stdlib"
)

type DBMetricsCollector struct {
	db *sql.DB
}

func NewDBMetricsCollector(db *sql.DB) *DBMetricsCollector {
	return &DBMetricsCollector{
		db: db,
	}
}

func (c *DBMetricsCollector) StartCollecting(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.collectMetrics()
			}
		}
	}()
}

func (c *DBMetricsCollector) collectMetrics() {
	stats := c.db.Stats()

	DBConnectionsActive.Set(float64(stats.InUse))
	DBConnectionsIdle.Set(float64(stats.Idle))
}

type TracedConnector struct {
	connector *stdlib.Driver
}

type TracedConn struct {
	*sql.Conn
}

type TracedStmt struct {
	*sql.Stmt
	query string
	table string
}

type TracedTx struct {
	*sql.Tx
}

func WrapDBWithMetrics(db *sql.DB) *sql.DB {
	return db
}

func InstrumentQuery(ctx context.Context, db *sql.DB, queryType, table, query string, args ...interface{}) (*sql.Rows, error) {
	end := TimeDBQuery(queryType, table)
	defer end()

	return db.QueryContext(ctx, query, args...)
}

func InstrumentExec(ctx context.Context, db *sql.DB, queryType, table, query string, args ...interface{}) (sql.Result, error) {
	end := TimeDBQuery(queryType, table)
	defer end()

	return db.ExecContext(ctx, query, args...)
}

func InstrumentQueryRow(ctx context.Context, db *sql.DB, queryType, table, query string, args ...interface{}) *sql.Row {
	end := TimeDBQuery(queryType, table)
	defer end()

	return db.QueryRowContext(ctx, query, args...)
}

func InstrumentTxQuery(ctx context.Context, tx *sql.Tx, queryType, table, query string, args ...interface{}) (*sql.Rows, error) {
	end := TimeDBQuery(queryType, table)
	defer end()

	return tx.QueryContext(ctx, query, args...)
}

func InstrumentTxExec(ctx context.Context, tx *sql.Tx, queryType, table, query string, args ...interface{}) (sql.Result, error) {
	end := TimeDBQuery(queryType, table)
	defer end()

	return tx.ExecContext(ctx, query, args...)
}

func InstrumentTxQueryRow(ctx context.Context, tx *sql.Tx, queryType, table, query string, args ...interface{}) *sql.Row {
	end := TimeDBQuery(queryType, table)
	defer end()

	return tx.QueryRowContext(ctx, query, args...)
}
