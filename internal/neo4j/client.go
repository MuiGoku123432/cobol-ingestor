package neo4j

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cobol-ingestor/internal/config"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// Client wraps a Neo4j driver with convenience methods.
type Client struct {
	driver   neo4j.DriverWithContext
	database string
	logger   *zap.Logger
}

// NewClient creates a new Neo4j client from config.
func NewClient(ctx context.Context, cfg config.Neo4jConfig, logger *zap.Logger) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.User, cfg.Password, ""))
	if err != nil {
		return nil, fmt.Errorf("creating neo4j driver: %w", err)
	}

	return &Client{
		driver:   driver,
		database: cfg.Database,
		logger:   logger,
	}, nil
}

// VerifyConnectivity checks the Neo4j connection is alive.
func (c *Client) VerifyConnectivity(ctx context.Context) error {
	return c.driver.VerifyConnectivity(ctx)
}

// RunMigrations reads .cypher files from migrationsDir and executes them.
func (c *Client) RunMigrations(ctx context.Context, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	// Sort to ensure deterministic order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.database})
	defer session.Close(ctx)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cypher") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}

		statements := strings.Split(string(data), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			c.logger.Debug("running migration statement", zap.String("file", entry.Name()), zap.String("cypher", stmt))
			_, err := session.Run(ctx, stmt, nil)
			if err != nil {
				return fmt.Errorf("executing migration %s: %w", entry.Name(), err)
			}
		}

		c.logger.Info("applied migration", zap.String("file", entry.Name()))
	}

	return nil
}

// NewSession creates a new Neo4j session.
func (c *Client) NewSession(ctx context.Context) neo4j.SessionWithContext {
	return c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.database})
}

// Close shuts down the Neo4j driver.
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}
