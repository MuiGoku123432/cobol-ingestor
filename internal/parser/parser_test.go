package parser

import (
	"testing"

	"cobol-ingestor/internal/graph"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleJSON = `{
  "programId": "CUSTMAINT",
  "copyReferences": ["CUSTFILE", "ERRHAND"],
  "callTargets": [
    {"target": "CUSTRPT", "isDynamic": false},
    {"target": "CUSTVAL", "isDynamic": true}
  ],
  "paragraphs": ["0000-MAIN", "1000-INIT", "2000-PROCESS", "9000-CLEANUP"],
  "sections": ["MAIN-LOGIC"],
  "fileDefinitions": [
    {"name": "CUSTOMER-FILE", "organization": "INDEXED"}
  ],
  "dataItems": [
    {"name": "WS-CUSTOMER-REC", "level": 1, "picture": ""},
    {"name": "WS-RETURN-CODE", "level": 77, "picture": "S9(4) COMP"}
  ],
  "sqlStatements": [
    {"type": "SELECT", "text": "SELECT CUST_NAME, CUST_ADDR FROM CUSTOMER WHERE CUST_ID = :WS-CUST-ID"}
  ],
  "cicsCommands": [
    {"command": "SEND MAP"}
  ]
}`

func TestParsePass1Response(t *testing.T) {
	result, err := ParsePass1Response(sampleJSON, "/src/CUSTMAINT.CBL")
	require.NoError(t, err)

	assert.Equal(t, "/src/CUSTMAINT.CBL", result.SourceFile)

	// Programs
	require.Len(t, result.Programs, 1)
	assert.Equal(t, "CUSTMAINT", result.Programs[0].ProgramID)
	assert.Equal(t, "COBOL", result.Programs[0].Language)

	// Copybooks
	require.Len(t, result.Copybooks, 2)
	assert.Equal(t, "CUSTFILE", result.Copybooks[0].Name)
	assert.Equal(t, "ERRHAND", result.Copybooks[1].Name)

	// Paragraphs
	require.Len(t, result.Paragraphs, 4)
	assert.Equal(t, "0000-MAIN", result.Paragraphs[0].Name)

	// Sections
	require.Len(t, result.Sections, 1)
	assert.Equal(t, "MAIN-LOGIC", result.Sections[0].Name)

	// File definitions
	require.Len(t, result.FileDefs, 1)
	assert.Equal(t, "CUSTOMER-FILE", result.FileDefs[0].Name)

	// Data items
	require.Len(t, result.DataItems, 2)
	assert.Equal(t, "WS-CUSTOMER-REC", result.DataItems[0].Name)
	assert.Equal(t, "CUSTMAINT.01.WS-CUSTOMER-REC", result.DataItems[0].FQN)

	// SQL
	require.Len(t, result.SQLStatements, 1)
	assert.Equal(t, "SELECT", result.SQLStatements[0].Type)

	// CICS
	require.Len(t, result.CICSTxns, 1)
	assert.Equal(t, "SEND MAP", result.CICSTxns[0].Command)

	// Relationships
	var relTypes []graph.RelType
	for _, r := range result.Relationships {
		relTypes = append(relTypes, r.Type)
	}
	assert.Contains(t, relTypes, graph.RelIncludes)
	assert.Contains(t, relTypes, graph.RelCalls)
	assert.Contains(t, relTypes, graph.RelBelongsTo)
	assert.Contains(t, relTypes, graph.RelExecutesSQL)
	assert.Contains(t, relTypes, graph.RelExecutesCICS)
}

func TestParsePass1Response_MarkdownFences(t *testing.T) {
	wrapped := "```json\n" + sampleJSON + "\n```"
	result, err := ParsePass1Response(wrapped, "/src/TEST.CBL")
	require.NoError(t, err)
	assert.Equal(t, "CUSTMAINT", result.Programs[0].ProgramID)
}

func TestParsePass1Response_InvalidJSON(t *testing.T) {
	_, err := ParsePass1Response("not json at all", "/src/BAD.CBL")
	assert.Error(t, err)
}
