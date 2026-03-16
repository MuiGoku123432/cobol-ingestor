package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	n4j "cobol-ingestor/internal/neo4j"

	"go.uber.org/zap"
)

// Neo4jService manages the Neo4j database connection for the desktop app.
type Neo4jService struct {
	app    *App
	client *n4j.Client
	reader n4j.Reader
}

// Neo4jStatus is returned to the frontend.
type Neo4jStatus struct {
	Connected bool   `json:"connected"`
	URI       string `json:"uri"`
	Database  string `json:"database"`
	Error     string `json:"error,omitempty"`
}

// DockerStatus reports Docker/Neo4j container state.
type DockerStatus struct {
	DockerAvailable bool   `json:"dockerAvailable"`
	ContainerRunning bool  `json:"containerRunning"`
	Error           string `json:"error,omitempty"`
}

// GetStatus returns the current Neo4j connection status.
func (s *Neo4jService) GetStatus() Neo4jStatus {
	return Neo4jStatus{
		Connected: s.reader != nil,
		URI:       s.app.cfg.Neo4j.URI,
		Database:  s.app.cfg.Neo4j.Database,
	}
}

// Connect establishes a connection to Neo4j with the given credentials.
func (s *Neo4jService) Connect(uri, user, password, database string) error {
	s.disconnect(context.Background())

	s.app.cfg.Neo4j.URI = uri
	s.app.cfg.Neo4j.User = user
	s.app.cfg.Neo4j.Password = password
	if database != "" {
		s.app.cfg.Neo4j.Database = database
	}

	ctx := s.app.ctx
	if err := s.tryConnect(ctx); err != nil {
		return err
	}

	// Re-initialize MCP with the new connection
	if err := s.app.initMCP(); err != nil {
		s.app.logger.Error("init MCP after neo4j connect", zap.Error(err))
	}

	return nil
}

// Disconnect closes the Neo4j connection.
func (s *Neo4jService) Disconnect() {
	s.disconnect(context.Background())
}

// TestConnection tests connectivity without persisting the connection.
func (s *Neo4jService) TestConnection(uri, user, password, database string) error {
	cfg := s.app.cfg.Neo4j
	cfg.URI = uri
	cfg.User = user
	cfg.Password = password
	if database != "" {
		cfg.Database = database
	}

	ctx := context.Background()
	client, err := n4j.NewClient(ctx, cfg, s.app.logger)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close(ctx)

	if err := client.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("connectivity check failed: %w", err)
	}
	return nil
}

// CheckDocker checks if Docker is available and if a Neo4j container is running.
func (s *Neo4jService) CheckDocker() DockerStatus {
	// Check Docker availability
	if err := exec.Command("docker", "info").Run(); err != nil {
		return DockerStatus{DockerAvailable: false, Error: "Docker not available"}
	}

	// Check if neo4j container is running via docker compose
	out, err := exec.Command("docker", "compose", "ps", "--services", "--filter", "status=running").Output()
	if err != nil {
		return DockerStatus{DockerAvailable: true, ContainerRunning: false}
	}

	running := strings.Contains(string(out), "neo4j")
	return DockerStatus{DockerAvailable: true, ContainerRunning: running}
}

// StartDocker starts Neo4j via docker compose.
func (s *Neo4jService) StartDocker() error {
	cmd := exec.Command("docker", "compose", "up", "-d", "neo4j")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up: %s: %w", string(out), err)
	}
	return nil
}

func (s *Neo4jService) tryConnect(ctx context.Context) error {
	client, err := n4j.NewClient(ctx, s.app.cfg.Neo4j, s.app.logger)
	if err != nil {
		return err
	}

	if err := client.VerifyConnectivity(ctx); err != nil {
		client.Close(ctx)
		return err
	}

	s.client = client
	s.reader = client
	s.app.logger.Info("neo4j connected", zap.String("uri", s.app.cfg.Neo4j.URI))
	return nil
}

func (s *Neo4jService) disconnect(ctx context.Context) {
	if s.client != nil {
		s.client.Close(ctx)
		s.client = nil
		s.reader = nil
	}
}
