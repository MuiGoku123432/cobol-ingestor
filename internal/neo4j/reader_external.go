package neo4j

import (
	"context"
	"encoding/json"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func (c *Client) ListExternalDBTables(ctx context.Context) ([]ExternalDBTableInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		records, err := tx.Run(ctx,
			"MATCH (t:ExternalDBTable) RETURN t.name AS name, t.schema AS schema, t.databaseName AS databaseName, t.databaseType AS databaseType, t.columns AS columns ORDER BY t.name",
			nil)
		if err != nil {
			return nil, err
		}

		var tables []ExternalDBTableInfo
		for records.Next(ctx) {
			r := records.Record()
			tables = append(tables, ExternalDBTableInfo{
				Name:         getStr(r, "name"),
				Schema:       getStr(r, "schema"),
				DatabaseName: getStr(r, "databaseName"),
				DatabaseType: getStr(r, "databaseType"),
				Columns:      getStr(r, "columns"),
			})
		}
		return tables, records.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]ExternalDBTableInfo), nil
}

func (c *Client) GetExternalDBMapping(ctx context.Context, tableName string) (*ExternalDBMappingInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		records, err := tx.Run(ctx,
			"MATCH (d:DBTable)-[r:MAPS_TO_EXT_DB]->(e:ExternalDBTable {name: $name}) "+
				"RETURN d.name AS cobolTable, e.name AS externalTable, r.confidence AS confidence, r.reason AS reason, r.columnMappings AS columnMappings LIMIT 1",
			map[string]any{"name": tableName})
		if err != nil {
			return nil, err
		}
		if !records.Next(ctx) {
			return nil, nil
		}
		r := records.Record()
		return &ExternalDBMappingInfo{
			CobolTable:     getStr(r, "cobolTable"),
			ExternalTable:  getStr(r, "externalTable"),
			Confidence:     getFloat64(r, "confidence"),
			Reason:         getStr(r, "reason"),
			ColumnMappings: getStr(r, "columnMappings"),
		}, records.Err()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.(*ExternalDBMappingInfo), nil
}

func (c *Client) GetCobolToExternalMappings(ctx context.Context, cobolTable string) ([]ExternalDBMappingInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		records, err := tx.Run(ctx,
			"MATCH (d:DBTable {name: $name})-[r:MAPS_TO_EXT_DB]->(e:ExternalDBTable) "+
				"RETURN d.name AS cobolTable, e.name AS externalTable, r.confidence AS confidence, r.reason AS reason, r.columnMappings AS columnMappings",
			map[string]any{"name": cobolTable})
		if err != nil {
			return nil, err
		}

		var mappings []ExternalDBMappingInfo
		for records.Next(ctx) {
			r := records.Record()
			mappings = append(mappings, ExternalDBMappingInfo{
				CobolTable:     getStr(r, "cobolTable"),
				ExternalTable:  getStr(r, "externalTable"),
				Confidence:     getFloat64(r, "confidence"),
				Reason:         getStr(r, "reason"),
				ColumnMappings: getStr(r, "columnMappings"),
			})
		}
		return mappings, records.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]ExternalDBMappingInfo), nil
}

func (c *Client) GetGapAnalysis(ctx context.Context) ([]GapInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		records, err := tx.Run(ctx,
			"MATCH (n:ExternalDatabase) WHERE n.gaps IS NOT NULL RETURN n.gaps AS gaps LIMIT 1",
			nil)
		if err != nil {
			return nil, err
		}
		if !records.Next(ctx) {
			return []GapInfo{}, nil
		}
		gapStr := getStr(records.Record(), "gaps")
		if gapStr == "" {
			return []GapInfo{}, nil
		}
		var gaps []GapInfo
		if err := json.Unmarshal([]byte(gapStr), &gaps); err != nil {
			return []GapInfo{}, nil
		}
		return gaps, records.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]GapInfo), nil
}

func (c *Client) GetDataFlowPaths(ctx context.Context, tableName string) ([]DataFlowPathInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		records, err := tx.Run(ctx,
			"MATCH (n:ExternalDatabase) WHERE n.dataFlows IS NOT NULL RETURN n.dataFlows AS flows LIMIT 1",
			nil)
		if err != nil {
			return nil, err
		}
		if !records.Next(ctx) {
			return []DataFlowPathInfo{}, nil
		}
		flowStr := getStr(records.Record(), "flows")
		if flowStr == "" {
			return []DataFlowPathInfo{}, nil
		}
		var flows []DataFlowPathInfo
		if err := json.Unmarshal([]byte(flowStr), &flows); err != nil {
			return []DataFlowPathInfo{}, nil
		}

		// Filter by table name if provided
		if tableName == "" {
			return flows, records.Err()
		}
		var filtered []DataFlowPathInfo
		for _, f := range flows {
			if f.DB2Table == tableName || f.ExternalTable == tableName {
				filtered = append(filtered, f)
			}
		}
		return filtered, records.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]DataFlowPathInfo), nil
}
