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

// GraphNode represents a node for the frontend graph visualization.
type GraphNode struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Caption string `json:"caption"`
	Size    int    `json:"size"`
	Color   string `json:"color"`
	Domain  string `json:"domain,omitempty"`
}

// GraphRelationship represents an edge for the frontend graph visualization.
type GraphRelationship struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Caption string `json:"caption"`
	Type    string `json:"type"`
}

// GraphData holds nodes and relationships for the frontend.
type GraphData struct {
	Nodes         []GraphNode         `json:"nodes"`
	Relationships []GraphRelationship `json:"relationships"`
}

var nodeColorMap = map[string]string{
	"Program":        "#238636",
	"Copybook":       "#58a6ff",
	"DataItem":       "#8b949e",
	"Paragraph":      "#d2a8ff",
	"BusinessDomain": "#f0883e",
	"JCLJob":         "#f85149",
}

var nodeSizeMap = map[string]int{
	"Program":        30,
	"Copybook":       20,
	"Paragraph":      15,
	"BusinessDomain": 35,
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

func nodeColor(label string) string {
	if c, ok := nodeColorMap[label]; ok {
		return c
	}
	return "#8b949e"
}

func nodeSize(label string) int {
	if sz, ok := nodeSizeMap[label]; ok {
		return sz
	}
	return 20
}

// GetCallGraph returns Programs connected by CALLS relationships.
// If domain is non-empty, only programs belonging to that domain are returned.
func (s *Neo4jService) GetCallGraph(domain string) (*GraphData, error) {
	if s.client == nil {
		return nil, fmt.Errorf("not connected to Neo4j")
	}
	ctx := context.Background()
	session := s.client.NewSession(ctx)
	defer session.Close(ctx)

	var query string
	params := map[string]any{}

	if domain != "" {
		query = `MATCH (p:Program)-[:BELONGS_TO]->(d:BusinessDomain {name: $domain})
WITH collect(p) AS domainProgs
UNWIND domainProgs AS p
OPTIONAL MATCH (p)-[r:CALLS]->(q:Program) WHERE q IN domainProgs
RETURN p.programId AS srcId, labels(p) AS srcLabels,
       q.programId AS tgtId, labels(q) AS tgtLabels,
       type(r) AS relType
LIMIT 200`
		params["domain"] = domain
	} else {
		query = `MATCH (p:Program)
OPTIONAL MATCH (p)-[r:CALLS]->(q:Program)
RETURN p.programId AS srcId, labels(p) AS srcLabels,
       q.programId AS tgtId, labels(q) AS tgtLabels,
       type(r) AS relType
LIMIT 200`
	}

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("call graph query: %w", err)
	}

	nodeMap := map[string]GraphNode{}
	var rels []GraphRelationship
	relIdx := 0

	for result.Next(ctx) {
		rec := result.Record()
		srcID, _ := rec.Get("srcId")
		if srcID == nil {
			continue
		}
		sid := fmt.Sprint(srcID)
		if _, exists := nodeMap[sid]; !exists {
			nodeMap[sid] = GraphNode{
				ID: sid, Label: "Program", Caption: sid,
				Size: nodeSize("Program"), Color: nodeColor("Program"),
				Domain: domain,
			}
		}

		tgtID, _ := rec.Get("tgtId")
		relType, _ := rec.Get("relType")
		if tgtID != nil && relType != nil {
			tid := fmt.Sprint(tgtID)
			if _, exists := nodeMap[tid]; !exists {
				nodeMap[tid] = GraphNode{
					ID: tid, Label: "Program", Caption: tid,
					Size: nodeSize("Program"), Color: nodeColor("Program"),
					Domain: domain,
				}
			}
			rels = append(rels, GraphRelationship{
				ID:      fmt.Sprintf("rel-%d", relIdx),
				From:    sid,
				To:      tid,
				Caption: fmt.Sprint(relType),
				Type:    fmt.Sprint(relType),
			})
			relIdx++
		}
	}

	nodes := make([]GraphNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}
	return &GraphData{Nodes: nodes, Relationships: rels}, nil
}

// GetNodeNeighbors returns 1-hop neighbors of a given node.
func (s *Neo4jService) GetNodeNeighbors(nodeID, nodeLabel string) (*GraphData, error) {
	if s.client == nil {
		return nil, fmt.Errorf("not connected to Neo4j")
	}
	ctx := context.Background()
	session := s.client.NewSession(ctx)
	defer session.Close(ctx)

	// Match outgoing and incoming relationships for the given node.
	query := fmt.Sprintf(
		`MATCH (n:%s {programId: $id})-[r]-(m)
RETURN n.programId AS srcId, labels(n) AS srcLabels,
       m.programId AS tgtId, coalesce(m.name, m.programId, m.jobName) AS tgtCaption,
       labels(m) AS tgtLabels,
       type(r) AS relType, startNode(r) = n AS outgoing
LIMIT 50`, nodeLabel)

	result, err := session.Run(ctx, query, map[string]any{"id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("neighbors query: %w", err)
	}

	nodeMap := map[string]GraphNode{}
	var rels []GraphRelationship
	relIdx := 0

	// Add the center node
	nodeMap[nodeID] = GraphNode{
		ID: nodeID, Label: nodeLabel, Caption: nodeID,
		Size: nodeSize(nodeLabel), Color: nodeColor(nodeLabel),
	}

	for result.Next(ctx) {
		rec := result.Record()
		tgtID, _ := rec.Get("tgtId")
		tgtCaption, _ := rec.Get("tgtCaption")
		tgtLabelsRaw, _ := rec.Get("tgtLabels")
		relType, _ := rec.Get("relType")
		outgoing, _ := rec.Get("outgoing")

		if tgtID == nil {
			continue
		}
		tid := fmt.Sprint(tgtID)
		caption := fmt.Sprint(tgtCaption)
		if caption == "<nil>" {
			caption = tid
		}

		tgtLabel := "Program"
		if labels, ok := tgtLabelsRaw.([]any); ok && len(labels) > 0 {
			tgtLabel = fmt.Sprint(labels[0])
		}

		if _, exists := nodeMap[tid]; !exists {
			nodeMap[tid] = GraphNode{
				ID: tid, Label: tgtLabel, Caption: caption,
				Size: nodeSize(tgtLabel), Color: nodeColor(tgtLabel),
			}
		}

		if relType != nil {
			from, to := nodeID, tid
			if outgoing != nil {
				if out, ok := outgoing.(bool); ok && !out {
					from, to = tid, nodeID
				}
			}
			rels = append(rels, GraphRelationship{
				ID:      fmt.Sprintf("rel-n-%d", relIdx),
				From:    from,
				To:      to,
				Caption: fmt.Sprint(relType),
				Type:    fmt.Sprint(relType),
			})
			relIdx++
		}
	}

	nodes := make([]GraphNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}
	return &GraphData{Nodes: nodes, Relationships: rels}, nil
}

// GetNodeDetail returns detailed information about a specific node.
func (s *Neo4jService) GetNodeDetail(nodeID, nodeLabel string) (map[string]any, error) {
	if s.reader == nil {
		return nil, fmt.Errorf("not connected to Neo4j")
	}
	ctx := context.Background()

	switch nodeLabel {
	case "Program":
		detail, err := s.reader.GetProgram(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type":          "Program",
			"programId":     detail.ProgramID,
			"filePath":      detail.FilePath,
			"language":      detail.Language,
			"lineCount":     detail.LineCount,
			"executionMode": detail.ExecutionMode,
			"deadCode":      detail.DeadCode,
			"riskScore":     detail.RiskScore,
			"riskType":      detail.RiskType,
			"callers":       detail.Callers,
			"callees":       detail.Callees,
			"copybooks":     detail.Copybooks,
			"paragraphs":    detail.Paragraphs,
		}, nil
	case "Copybook":
		usage, err := s.reader.GetCopybookUsage(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type":     "Copybook",
			"name":     usage.Name,
			"programs": usage.Programs,
		}, nil
	case "BusinessDomain":
		domain, err := s.reader.GetBusinessDomain(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type":        "BusinessDomain",
			"name":        domain.Name,
			"description": domain.Description,
			"programs":    domain.Programs,
		}, nil
	default:
		return map[string]any{
			"type": nodeLabel,
			"id":   nodeID,
		}, nil
	}
}

// GetBusinessDomains returns the list of business domains for the filter dropdown.
func (s *Neo4jService) GetBusinessDomains() ([]map[string]string, error) {
	if s.reader == nil {
		return nil, fmt.Errorf("not connected to Neo4j")
	}
	ctx := context.Background()
	domains, err := s.reader.ListBusinessDomains(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]string, len(domains))
	for i, d := range domains {
		result[i] = map[string]string{
			"name":        d.Name,
			"description": d.Description,
		}
	}
	return result, nil
}
