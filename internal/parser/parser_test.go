package parser

import (
	"testing"

	"cobol-ingestor/internal/graph"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleJSON = `{
  "programId": "CUSTMAINT",
  "executionMode": "CICS",
  "copyReferences": ["CUSTFILE", "ERRHAND"],
  "callTargets": [
    {"target": "CUSTRPT", "isDynamic": false},
    {"target": "CUSTVAL", "isDynamic": true}
  ],
  "paragraphs": ["0000-MAIN", "1000-INIT", "2000-PROCESS", "9000-CLEANUP"],
  "sections": ["MAIN-LOGIC"],
  "fileDefinitions": [
    {"name": "CUSTOMER-FILE", "organization": "INDEXED", "vsamType": "KSDS", "dataStoreType": "VSAM"}
  ],
  "dataItems": [
    {"name": "WS-CUSTOMER-REC", "level": 1, "picture": "", "usage": ""},
    {"name": "WS-RETURN-CODE", "level": 77, "picture": "S9(4) COMP", "usage": "COMP"}
  ],
  "conditions": [
    {"name": "VALID-STATUS", "parent": "WS-STATUS-CODE", "value": "'Y'"},
    {"name": "EOF-REACHED", "parent": "WS-EOF-FLAG", "value": "'Y'"}
  ],
  "parameters": [
    {"name": "LK-CUST-ID", "level": 1, "direction": "IN"},
    {"name": "LK-RESULT", "level": 1, "direction": "OUT"}
  ],
  "sqlStatements": [
    {"type": "SELECT", "text": "SELECT CUST_NAME, CUST_ADDR FROM CUSTOMER WHERE CUST_ID = :WS-CUST-ID", "targetTable": "CUSTOMER"}
  ],
  "cicsCommands": [
    {"command": "SEND MAP"}
  ],
  "externalInterfaces": [
    {"type": "CICS_LINK", "details": "LINK PROGRAM('CUSTRPT')", "paragraph": "2000-PROCESS"},
    {"type": "MQ", "details": "MQPUT to CUST.UPDATE.Q", "paragraph": "2000-PROCESS"},
    {"type": "IMS", "details": "CBLTDLI GU on CUSTOMER-PCB", "paragraph": "1000-INIT"}
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
	assert.Equal(t, "CICS", result.Programs[0].ExecutionMode)

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
	assert.Equal(t, "KSDS", result.FileDefs[0].VSAMType)
	assert.Equal(t, "VSAM", result.FileDefs[0].DataStoreType)

	// Data items
	require.Len(t, result.DataItems, 2)
	assert.Equal(t, "WS-CUSTOMER-REC", result.DataItems[0].Name)
	assert.Equal(t, "CUSTMAINT.01.WS-CUSTOMER-REC", result.DataItems[0].FQN)
	assert.Equal(t, "COMP", result.DataItems[1].Usage)

	// Conditions
	require.Len(t, result.Conditions, 2)
	assert.Equal(t, "VALID-STATUS", result.Conditions[0].Name)
	assert.Equal(t, "WS-STATUS-CODE", result.Conditions[0].Parent)
	assert.Equal(t, "'Y'", result.Conditions[0].Value)

	// Parameters
	require.Len(t, result.Parameters, 2)
	assert.Equal(t, "LK-CUST-ID", result.Parameters[0].Name)
	assert.Equal(t, "IN", result.Parameters[0].Direction)
	assert.Equal(t, "LK-RESULT", result.Parameters[1].Name)
	assert.Equal(t, "OUT", result.Parameters[1].Direction)

	// SQL
	require.Len(t, result.SQLStatements, 1)
	assert.Equal(t, "SELECT", result.SQLStatements[0].Type)
	assert.Equal(t, "CUSTOMER", result.SQLStatements[0].TargetTable)

	// CICS
	require.Len(t, result.CICSTxns, 1)
	assert.Equal(t, "SEND MAP", result.CICSTxns[0].Command)

	// Conditions FQN
	assert.Equal(t, "CUSTMAINT.VALID-STATUS", result.Conditions[0].FQN)

	// Parameters FQN
	assert.Equal(t, "CUSTMAINT.LK-CUST-ID", result.Parameters[0].FQN)

	// External interfaces
	require.Len(t, result.ExternalInterfaces, 3)
	assert.Equal(t, "CICS_LINK", result.ExternalInterfaces[0].Type)
	assert.Equal(t, "LINK PROGRAM('CUSTRPT')", result.ExternalInterfaces[0].Details)
	assert.Equal(t, "2000-PROCESS", result.ExternalInterfaces[0].Paragraph)
	assert.Equal(t, "MQ", result.ExternalInterfaces[1].Type)
	assert.Equal(t, "IMS", result.ExternalInterfaces[2].Type)
	assert.Equal(t, "1000-INIT", result.ExternalInterfaces[2].Paragraph)
	assert.Equal(t, "CUSTMAINT", result.ExternalInterfaces[0].ProgramID)

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
	assert.Contains(t, relTypes, graph.RelConditionOf)
	assert.Contains(t, relTypes, graph.RelParameterOf)
	assert.Contains(t, relTypes, graph.RelExternalInterface)
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

func TestParsePass1Response_ExternalInterfaces(t *testing.T) {
	jsonStr := `{
		"programId": "INTFTEST",
		"executionMode": "CICS",
		"copyReferences": [],
		"callTargets": [],
		"paragraphs": ["1000-MQ", "2000-CICS", "3000-IMS", "4000-IDMS", "5000-ADABAS", "6000-SORT", "7000-BATCH"],
		"sections": [],
		"fileDefinitions": [],
		"dataItems": [],
		"conditions": [],
		"parameters": [],
		"sqlStatements": [],
		"cicsCommands": [],
		"externalInterfaces": [
			{"type": "MQ", "details": "MQPUT to ORDER.Q", "paragraph": "1000-MQ"},
			{"type": "CICS_LINK", "details": "LINK PROGRAM('SUBPROG1')", "paragraph": "2000-CICS"},
			{"type": "CICS_XCTL", "details": "XCTL PROGRAM('NEXTPROG')", "paragraph": "2000-CICS"},
			{"type": "CICS_TS", "details": "WRITEQ TS QUEUE('TEMPQ')", "paragraph": "2000-CICS"},
			{"type": "CICS_TD", "details": "WRITEQ TD QUEUE('LOGQ')", "paragraph": "2000-CICS"},
			{"type": "CICS_START", "details": "START TRANSID('TXN1')", "paragraph": "2000-CICS"},
			{"type": "CICS_FILE", "details": "READ FILE('CUSTFILE') INTO(CUST-REC)", "paragraph": "2000-CICS"},
			{"type": "CICS_ENQ", "details": "ENQ RESOURCE('CUST-LOCK') LENGTH(10)", "paragraph": "2000-CICS"},
			{"type": "IMS", "details": "CBLTDLI GU CUSTOMER-PCB", "paragraph": "3000-IMS"},
			{"type": "IDMS", "details": "OBTAIN CALC CUSTOMER-RECORD", "paragraph": "4000-IDMS"},
			{"type": "ADABAS", "details": "CALL ADABAS command L3 file 20", "paragraph": "5000-ADABAS"},
			{"type": "SORT", "details": "SORT SORT-FILE ON ASCENDING KEY SORT-KEY INPUT PROCEDURE 6100-INPUT", "paragraph": "6000-SORT"},
			{"type": "BATCH_UTIL", "details": "CALL DFSORT for inline sort", "paragraph": "7000-BATCH"},
			{"type": "TCP", "details": "Socket call to external service", "paragraph": "1000-MQ"},
			{"type": "FILE_TRANSFER", "details": "FTP transfer of report file", "paragraph": "1000-MQ"}
		]
	}`

	result, err := ParsePass1Response(jsonStr, "/src/INTFTEST.CBL")
	require.NoError(t, err)

	// All 15 interface types parsed
	require.Len(t, result.ExternalInterfaces, 15)

	expectedTypes := []string{
		"MQ", "CICS_LINK", "CICS_XCTL", "CICS_TS", "CICS_TD", "CICS_START",
		"CICS_FILE", "CICS_ENQ", "IMS", "IDMS", "ADABAS", "SORT", "BATCH_UTIL",
		"TCP", "FILE_TRANSFER",
	}
	for i, expected := range expectedTypes {
		assert.Equal(t, expected, result.ExternalInterfaces[i].Type, "interface %d type", i)
		assert.NotEmpty(t, result.ExternalInterfaces[i].Details, "interface %d details", i)
		assert.NotEmpty(t, result.ExternalInterfaces[i].Paragraph, "interface %d paragraph", i)
		assert.Equal(t, "INTFTEST", result.ExternalInterfaces[i].ProgramID, "interface %d programID", i)
	}

	// Verify HAS_INTERFACE relationships created
	hasInterfaceCount := 0
	for _, r := range result.Relationships {
		if r.Type == graph.RelExternalInterface {
			hasInterfaceCount++
			assert.Equal(t, "Program", r.FromLabel)
			assert.Equal(t, "INTFTEST", r.FromKey)
			assert.Equal(t, "ExternalInterface", r.ToLabel)
		}
	}
	assert.Equal(t, 15, hasInterfaceCount)
}
