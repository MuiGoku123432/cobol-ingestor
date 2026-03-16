package mcp

import (
	"context"
	"encoding/json"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerExternalDBTools(s *mcp.Server, reader n4j.Reader) {
	registerListExternalDBTables(s, reader)
	registerGetExternalDBMapping(s, reader)
	registerGetCobolToExternalMappings(s, reader)
	registerGetGapAnalysis(s, reader)
	registerGetDataFlowPaths(s, reader)
}

func registerListExternalDBTables(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Tables []n4j.ExternalDBTableInfo `json:"tables"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_external_db_tables",
		Description: "List all external database tables discovered during gap analysis, including schema, database name, and column details.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		tables, err := reader.ListExternalDBTables(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Tables: tables}, nil
	})
}

func registerGetExternalDBMapping(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_external_db_mapping",
		Description: "Get the COBOL DB2 table mapping for a given external database table, including confidence score and column mappings.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetExternalDBMappingInput) (*mcp.CallToolResult, any, error) {
		mapping, err := reader.GetExternalDBMapping(ctx, input.TableName)
		if err != nil {
			return nil, nil, err
		}
		if mapping == nil {
			return toolError("no mapping found for external table: " + input.TableName), nil, nil
		}
		return marshalResult(mapping)
	})
}

func registerGetCobolToExternalMappings(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Mappings []n4j.ExternalDBMappingInfo `json:"mappings"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_cobol_to_external_mappings",
		Description: "Get all external database table mappings for a given COBOL DB2 table, showing where COBOL data maps to in the external database.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetCobolToExternalMappingsInput) (*mcp.CallToolResult, *output, error) {
		mappings, err := reader.GetCobolToExternalMappings(ctx, input.CobolTable)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Mappings: mappings}, nil
	})
}

func registerGetGapAnalysis(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_gap_analysis",
		Description: "Get the gap analysis results showing tables/columns that exist only in COBOL (cobol_only) or only in the external database (external_only).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, any, error) {
		gaps, err := reader.GetGapAnalysis(ctx)
		if err != nil {
			return nil, nil, err
		}
		b, err := json.Marshal(map[string]any{"gaps": gaps})
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})
}

func registerGetDataFlowPaths(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Flows []n4j.DataFlowPathInfo `json:"flows"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_data_flow_paths",
		Description: "Get end-to-end data flow paths showing how data moves from COBOL programs through DB2 tables to external database tables. Optionally filter by table name.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetDataFlowPathsInput) (*mcp.CallToolResult, *output, error) {
		flows, err := reader.GetDataFlowPaths(ctx, input.TableName)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Flows: flows}, nil
	})
}
