package neo4j

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// BWEntityInfo holds summary information about a BW entity for synthesis.
type BWEntityInfo struct {
	MergeID     string
	Name        string
	EntityType  string
	Description string
	SourceFile  string
}

// QueryBWEntitiesForSynthesis returns all BW entities ordered by source file for batching.
func (c *Client) QueryBWEntitiesForSynthesis(ctx context.Context) ([]BWEntityInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (e:BWEntity) "+
			"RETURN e.mergeId AS mergeId, e.name AS name, e.entityType AS entityType, "+
			"  e.description AS description, e.sourceFile AS sourceFile "+
			"ORDER BY e.sourceFile, e.name",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying BW entities for synthesis: %w", err)
	}

	var entities []BWEntityInfo
	for result.Next(ctx) {
		rec := result.Record()
		entities = append(entities, BWEntityInfo{
			MergeID:     getStr(rec, "mergeId"),
			Name:        getStr(rec, "name"),
			EntityType:  getStr(rec, "entityType"),
			Description: getStr(rec, "description"),
			SourceFile:  getStr(rec, "sourceFile"),
		})
	}
	return entities, nil
}

// UnlinkedBWService represents a BWEntity with zero COBOL references.
type UnlinkedBWService struct {
	MergeID     string
	Name        string
	EntityType  string
	Description string
	SourceFile  string
}

// QueryUnlinkedBWServices finds BWEntity nodes of service-like types with zero BW_REFERENCES.
func (c *Client) QueryUnlinkedBWServices(ctx context.Context) ([]UnlinkedBWService, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (e:BWEntity) "+
			"WHERE e.entityType IN ['Service', 'Endpoint', 'Controller', 'Adapter', 'Gateway', 'Proxy'] "+
			"AND NOT (e)-[:BW_REFERENCES]->() "+
			"RETURN e.mergeId AS mergeId, e.name AS name, e.entityType AS entityType, "+
			"  e.description AS description, e.sourceFile AS sourceFile "+
			"ORDER BY e.name",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying unlinked BW services: %w", err)
	}

	var services []UnlinkedBWService
	for result.Next(ctx) {
		rec := result.Record()
		services = append(services, UnlinkedBWService{
			MergeID:     getStr(rec, "mergeId"),
			Name:        getStr(rec, "name"),
			EntityType:  getStr(rec, "entityType"),
			Description: getStr(rec, "description"),
			SourceFile:  getStr(rec, "sourceFile"),
		})
	}
	return services, nil
}

// BWOrphanFile represents a BWFile where no child entity has BW_REFERENCES.
type BWOrphanFile struct {
	Path     string
	FileType string
	Summary  string
}

// QueryBWOrphanFiles finds BWFile nodes where no child BWEntity has any BW_REFERENCES.
func (c *Client) QueryBWOrphanFiles(ctx context.Context) ([]BWOrphanFile, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (f:BWFile)-[:BW_CONTAINS]->(e:BWEntity) "+
			"WITH f, collect(e) AS entities "+
			"WHERE none(e IN entities WHERE (e)-[:BW_REFERENCES]->()) "+
			"AND size(entities) > 0 "+
			"RETURN f.path AS path, f.fileType AS fileType, f.summary AS summary "+
			"ORDER BY f.path",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying BW orphan files: %w", err)
	}

	var files []BWOrphanFile
	for result.Next(ctx) {
		rec := result.Record()
		files = append(files, BWOrphanFile{
			Path:     getStr(rec, "path"),
			FileType: getStr(rec, "fileType"),
			Summary:  getStr(rec, "summary"),
		})
	}
	return files, nil
}

// BWFuzzyCandidate represents a fuzzy name match between a BWEntity and a COBOL program.
type BWFuzzyCandidate struct {
	EntityMergeID string
	EntityName    string
	ProgramID     string
	MatchReason   string
}

// QueryBWFuzzyMatches finds BWEntity names similar to COBOL program IDs using basic string patterns.
// Uses Cypher string functions for substring/prefix matching (no external fuzzy library needed).
func (c *Client) QueryBWFuzzyMatches(ctx context.Context) ([]BWFuzzyCandidate, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		// Match entities not already linked to the candidate program
		"MATCH (e:BWEntity) "+
			"WHERE NOT (e)-[:BW_REFERENCES]->() "+
			"MATCH (p:Program) "+
			"WHERE p.filePath IS NOT NULL "+
			// Fuzzy matching: entity name contains program ID or vice versa (>= 4 chars)
			"AND ("+
			"  (size(p.programId) >= 4 AND toUpper(e.name) CONTAINS toUpper(p.programId)) "+
			"  OR (size(e.name) >= 4 AND toUpper(p.programId) CONTAINS toUpper(replace(replace(e.name, 'Service', ''), 'Controller', ''))) "+
			") "+
			"RETURN e.mergeId AS mergeId, e.name AS entityName, p.programId AS programId, "+
			"  'name_contains' AS matchReason "+
			"LIMIT 100",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying BW fuzzy matches: %w", err)
	}

	var candidates []BWFuzzyCandidate
	for result.Next(ctx) {
		rec := result.Record()
		candidates = append(candidates, BWFuzzyCandidate{
			EntityMergeID: getStr(rec, "mergeId"),
			EntityName:    getStr(rec, "entityName"),
			ProgramID:     getStr(rec, "programId"),
			MatchReason:   getStr(rec, "matchReason"),
		})
	}

	c.logger.Info("found BW fuzzy match candidates", zap.Int("count", len(candidates)))
	return candidates, nil
}

// QueryCandidateProgramsForBWRepair returns COBOL programs relevant to a set of BW service entities,
// filtered by domain and interface type relevance.
func (c *Client) QueryCandidateProgramsForBWRepair(ctx context.Context) (string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program) "+
			"WHERE EXISTS { MATCH (e:ExternalInterface {programId: p.programId}) } "+
			"OR EXISTS { MATCH (p)-[:READS|WRITES]->(:File) } "+
			"RETURN p.programId AS pid, p.executionMode AS mode "+
			"ORDER BY pid LIMIT 200",
		nil)
	if err != nil {
		return "", fmt.Errorf("querying candidate programs: %w", err)
	}

	var sb string
	for result.Next(ctx) {
		rec := result.Record()
		sb += fmt.Sprintf("%s (%s)\n", getStr(rec, "pid"), getStr(rec, "mode"))
	}
	return sb, nil
}
