package graph

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Config holds Neo4j connection parameters.
type Config struct {
	URI      string
	Username string
	Password string
	Database string
}

// ConfigFromEnv returns a Config populated from environment variables,
// falling back to the provided defaults.
func ConfigFromEnv(defaults Config) Config {
	if v := os.Getenv("MUSICDL_GRAPH_URI"); v != "" {
		defaults.URI = v
	}
	if v := os.Getenv("MUSICDL_GRAPH_USER"); v != "" {
		defaults.Username = v
	}
	if v := os.Getenv("MUSICDL_GRAPH_PASSWORD"); v != "" {
		defaults.Password = v
	}
	return defaults
}

// Client wraps a Neo4j driver and provides graph memory operations.
type Client struct {
	driver   neo4j.DriverWithContext
	database string
	mu       sync.RWMutex
}

// NewClient creates a Client and verifies connectivity.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j driver creation failed: %w", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(connectCtx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("neo4j connectivity check failed: %w", err)
	}

	db := cfg.Database
	if db == "" {
		db = "neo4j"
	}

	c := &Client{driver: driver, database: db}

	if err := c.initSchema(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("graph schema initialization failed: %w", err)
	}

	log.Printf("INFO: graph_memory connected to %s (database=%s)", cfg.URI, db)
	return c, nil
}

// Close shuts down the driver.
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// session opens a new session for the configured database.
func (c *Client) session(ctx context.Context) neo4j.SessionWithContext {
	return c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.database})
}

// writeTransaction runs a write transaction with automatic retries.
func (c *Client) writeTransaction(ctx context.Context, work neo4j.ManagedTransactionWork) error {
	session := c.session(ctx)
	defer func() { _ = session.Close(ctx) }()
	_, err := session.ExecuteWrite(ctx, work)
	return err
}

// readTransaction runs a read transaction with automatic retries.
func (c *Client) readTransaction(ctx context.Context, work neo4j.ManagedTransactionWork) (any, error) {
	session := c.session(ctx)
	defer func() { _ = session.Close(ctx) }()
	return session.ExecuteRead(ctx, work)
}
