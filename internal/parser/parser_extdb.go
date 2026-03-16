package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"cobol-ingestor/internal/graph"
)

// ExternalDBParsedResult wraps graph.ExternalDBResult — returned by ParseExternalDBResponse.
type ExternalDBParsedResult = graph.ExternalDBResult

// extDBRawResponse is the JSON shape the LLM produces.
type extDBRawResponse struct {
	ExternalTables []extDBRawTable   `json:"externalTables"`
	Mappings       []extDBRawMapping `json:"mappings"`
	Gaps           []extDBRawGap     `json:"gaps"`
	DataFlows      []extDBRawFlow    `json:"dataFlows"`
}

type extDBRawTable struct {
	Name    string            `json:"name"`
	Schema  string            `json:"schema"`
	Columns []extDBRawColumn  `json:"columns"`
}

type extDBRawColumn struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	Nullable bool   `json:"nullable"`
	IsPK     bool   `json:"isPK"`
}

type extDBRawMapping struct {
	CobolTable     string                `json:"cobolTable"`
	ExternalTable  string                `json:"externalTable"`
	Confidence     float64               `json:"confidence"`
	Reason         string                `json:"reason"`
	ColumnMappings []extDBRawColMapping  `json:"columnMappings"`
}

type extDBRawColMapping struct {
	CobolColumn    string `json:"cobolColumn"`
	ExternalColumn string `json:"externalColumn"`
	Transform      string `json:"transform"`
}

type extDBRawGap struct {
	Side        string `json:"side"`
	TableName   string `json:"tableName"`
	ColumnName  string `json:"columnName"`
	Description string `json:"description"`
}

type extDBRawFlow struct {
	CobolProgram  string `json:"cobolProgram"`
	Operation     string `json:"operation"`
	DB2Table      string `json:"db2Table"`
	ExternalTable string `json:"externalTable"`
	FlowType      string `json:"flowType"`
	Description   string `json:"description"`
}

// jsonBlockRE matches a fenced JSON code block or bare JSON object.
var jsonBlockRE = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// ParseExternalDBResponse parses the LLM's JSON response into an ExternalDBResult.
func ParseExternalDBResponse(text, dbName, dbType string) (*graph.ExternalDBResult, error) {
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM response (length %d)", len(text))
	}

	var raw extDBRawResponse
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("parsing external DB JSON: %w", err)
	}

	result := &graph.ExternalDBResult{
		Database: graph.ExternalDatabase{
			ID:           strings.ToLower(dbName),
			Name:         dbName,
			DatabaseType: dbType,
		},
	}

	for _, t := range raw.ExternalTables {
		tbl := graph.ExternalDBTable{
			ID:           fmt.Sprintf("%s.%s.%s", strings.ToLower(dbName), strings.ToLower(t.Schema), strings.ToLower(t.Name)),
			Name:         t.Name,
			Schema:       t.Schema,
			DatabaseName: dbName,
			DatabaseType: dbType,
		}
		for _, c := range t.Columns {
			tbl.Columns = append(tbl.Columns, graph.ExtDBColumn{
				Name:     c.Name,
				DataType: c.DataType,
				Nullable: c.Nullable,
				IsPK:     c.IsPK,
			})
		}
		result.Tables = append(result.Tables, tbl)
	}

	for _, m := range raw.Mappings {
		mapping := graph.DBTableMapping{
			CobolDBTable:  m.CobolTable,
			ExternalTable: m.ExternalTable,
			Confidence:    m.Confidence,
			Reason:        m.Reason,
		}
		for _, cm := range m.ColumnMappings {
			mapping.ColumnMappings = append(mapping.ColumnMappings, graph.ColumnMapping{
				CobolColumn:    cm.CobolColumn,
				ExternalColumn: cm.ExternalColumn,
				Transform:      cm.Transform,
			})
		}
		result.Mappings = append(result.Mappings, mapping)
	}

	for _, g := range raw.Gaps {
		result.Gaps = append(result.Gaps, graph.GapInfo{
			Side:        g.Side,
			TableName:   g.TableName,
			ColumnName:  g.ColumnName,
			Description: g.Description,
		})
	}

	for _, f := range raw.DataFlows {
		result.Flows = append(result.Flows, graph.DataFlowPath{
			CobolProgram:  f.CobolProgram,
			Operation:     f.Operation,
			DB2Table:      f.DB2Table,
			ExternalTable: f.ExternalTable,
			FlowType:      f.FlowType,
			Description:   f.Description,
		})
	}

	return result, nil
}

// extractJSON tries to find JSON in the text, first in a code block, then as bare JSON.
func extractJSON(text string) string {
	// Try fenced code block first
	if matches := jsonBlockRE.FindStringSubmatch(text); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find bare JSON object
	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(text, "}")
	if end <= start {
		return ""
	}
	return strings.TrimSpace(text[start : end+1])
}
