// Package ruvector provides PostgreSQL/pgvector integration for vector operations.
package ruvector

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Connection manages PostgreSQL database connections.
type Connection struct {
	mu       sync.RWMutex
	db       *sql.DB
	config   RuVectorConfig
	connStr  string
}

// NewConnection creates a new PostgreSQL connection manager.
func NewConnection(config RuVectorConfig) *Connection {
	// Override from environment if not set
	if config.Host == "" {
		config.Host = getEnvOrDefault("PGHOST", "localhost")
	}
	if config.Port == 0 {
		config.Port = 5432
	}
	if config.User == "" {
		config.User = getEnvOrDefault("PGUSER", "postgres")
	}
	if config.Password == "" {
		config.Password = os.Getenv("PGPASSWORD")
	}
	if config.Database == "" {
		config.Database = os.Getenv("PGDATABASE")
	}
	if config.Schema == "" {
		config.Schema = "claude_flow"
	}

	return &Connection{
		config:  config,
		connStr: buildConnectionString(config),
	}
}

// buildConnectionString constructs a PostgreSQL connection string.
func buildConnectionString(config RuVectorConfig) string {
	sslMode := "disable"
	if config.SSL {
		sslMode = "require"
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Database, sslMode,
	)

	if config.Password != "" {
		connStr += fmt.Sprintf(" password=%s", config.Password)
	}

	return connStr
}

// Connect establishes a connection to PostgreSQL.
func (c *Connection) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		return nil // Already connected
	}

	db, err := sql.Open("postgres", c.connStr)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	c.db = db
	return nil
}

// Close closes the database connection.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		err := c.db.Close()
		c.db = nil
		return err
	}
	return nil
}

// DB returns the underlying database connection.
func (c *Connection) DB() *sql.DB {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db
}

// Config returns the connection configuration.
func (c *Connection) Config() RuVectorConfig {
	return c.config
}

// IsConnected returns whether a connection is established.
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db != nil
}

// Ping tests the database connection.
func (c *Connection) Ping(ctx context.Context) (time.Duration, error) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db == nil {
		return 0, fmt.Errorf("not connected")
	}

	start := time.Now()
	if err := db.PingContext(ctx); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// Exec executes a query without returning rows.
func (c *Connection) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("not connected")
	}

	return db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (c *Connection) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("not connected")
	}

	return db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row.
func (c *Connection) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db == nil {
		return nil
	}

	return db.QueryRowContext(ctx, query, args...)
}

// BeginTx starts a transaction.
func (c *Connection) BeginTx(ctx context.Context) (*sql.Tx, error) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("not connected")
	}

	return db.BeginTx(ctx, nil)
}

// GetPostgresVersion returns the PostgreSQL version.
func (c *Connection) GetPostgresVersion(ctx context.Context) (string, error) {
	var version string
	err := c.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", err
	}
	return version, nil
}

// GetPgVectorVersion returns the pgvector extension version.
func (c *Connection) GetPgVectorVersion(ctx context.Context) (string, error) {
	var version string
	err := c.QueryRow(ctx, `
		SELECT extversion 
		FROM pg_extension 
		WHERE extname = 'vector'
	`).Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Not installed
		}
		return "", err
	}
	return version, nil
}

// SchemaExists checks if the schema exists.
func (c *Connection) SchemaExists(ctx context.Context) (bool, error) {
	var exists bool
	err := c.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.schemata 
			WHERE schema_name = $1
		)
	`, c.config.Schema).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// TableExists checks if a table exists in the schema.
func (c *Connection) TableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	err := c.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = $1 AND table_name = $2
		)
	`, c.config.Schema, tableName).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
