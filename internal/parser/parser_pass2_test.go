package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplePass2JSON = `{
  "performs": [
    {
      "fromParagraph": "0000-MAIN",
      "toParagraph": "1000-INIT",
      "thruParagraph": "",
      "isLoop": false,
      "condition": ""
    },
    {
      "fromParagraph": "2000-PROCESS",
      "toParagraph": "2100-READ-NEXT",
      "thruParagraph": "2199-READ-EXIT",
      "isLoop": true,
      "condition": "UNTIL WS-EOF = 'Y'"
    }
  ],
  "dataFlows": [
    {
      "fromItem": "WS-INPUT-REC",
      "toItem": "WS-OUTPUT-REC",
      "context": "2000-PROCESS"
    }
  ],
  "fileOperations": [
    {
      "operation": "OPEN",
      "fileName": "CUSTOMER-FILE",
      "paragraph": "1000-INIT"
    },
    {
      "operation": "READ",
      "fileName": "CUSTOMER-FILE",
      "paragraph": "2100-READ-NEXT"
    }
  ],
  "sqlStatements": [
    {
      "type": "SELECT",
      "text": "SELECT CUST_NAME FROM CUSTOMER WHERE CUST_ID = :WS-ID"
    }
  ],
  "cicsCommands": [
    {
      "command": "SEND MAP"
    }
  ],
  "dataHierarchy": [
    {
      "name": "WS-CUSTOMER-REC",
      "level": 1,
      "parent": "",
      "picture": "",
      "copybook": "CUSTREC"
    },
    {
      "name": "WS-CUST-ID",
      "level": 5,
      "parent": "WS-CUSTOMER-REC",
      "picture": "X(10)",
      "copybook": "CUSTREC"
    }
  ],
  "redefines": [
    {
      "item": "WS-CUST-NUM",
      "redefines": "WS-CUST-ID"
    }
  ],
  "copybookDefinitions": [
    {
      "dataItem": "WS-CUSTOMER-REC",
      "copybook": "CUSTREC"
    }
  ],
  "annotations": [
    {
      "paragraph": "0000-MAIN",
      "description": "Main control paragraph that orchestrates initialization, processing, and cleanup.",
      "category": "PROCESSING"
    },
    {
      "paragraph": "1000-INIT",
      "description": "Opens files and initializes working storage variables.",
      "category": "INIT"
    }
  ],
  "conditionalLogic": [
    {
      "paragraph": "2000-PROCESS",
      "condition": "IF WS-STATUS = SPACES",
      "variables": ["WS-STATUS"],
      "type": "IF"
    }
  ],
  "dynamicCallResolution": [
    {
      "variable": "WS-PROG-NAME",
      "resolvedTargets": ["CUSTRPT", "CUSTVAL"],
      "paragraph": "2000-PROCESS"
    }
  ],
  "errorHandling": [
    {
      "paragraph": "9000-ERROR",
      "pattern": "FILE-STATUS",
      "details": "Checks FILE-STATUS after each I/O operation"
    }
  ]
}`

func TestParsePass2Response(t *testing.T) {
	result, err := ParsePass2Response(samplePass2JSON, "/src/CUSTMAINT.CBL", "CUSTMAINT")
	require.NoError(t, err)

	assert.Equal(t, "/src/CUSTMAINT.CBL", result.SourceFile)
	assert.Equal(t, "CUSTMAINT", result.ProgramID)

	// Performs
	require.Len(t, result.Performs, 2)
	assert.Equal(t, "0000-MAIN", result.Performs[0].FromParagraph)
	assert.Equal(t, "1000-INIT", result.Performs[0].ToParagraph)
	assert.False(t, result.Performs[0].IsLoop)
	assert.Equal(t, "2199-READ-EXIT", result.Performs[1].ThruParagraph)
	assert.True(t, result.Performs[1].IsLoop)

	// Data flows
	require.Len(t, result.DataFlows, 1)
	assert.Equal(t, "WS-INPUT-REC", result.DataFlows[0].FromItem)

	// File operations
	require.Len(t, result.FileOps, 2)
	assert.Equal(t, "OPEN", result.FileOps[0].Operation)
	assert.Equal(t, "READ", result.FileOps[1].Operation)

	// SQL
	require.Len(t, result.SQLDetails, 1)
	assert.Equal(t, "SELECT", result.SQLDetails[0].Type)

	// CICS
	require.Len(t, result.CICSDetails, 1)
	assert.Equal(t, "SEND MAP", result.CICSDetails[0].Command)

	// Data hierarchy
	require.Len(t, result.DataHierarchy, 2)
	assert.Equal(t, "WS-CUSTOMER-REC", result.DataHierarchy[0].Name)
	assert.Equal(t, "CUSTREC", result.DataHierarchy[0].Copybook)
	assert.Equal(t, "WS-CUSTOMER-REC", result.DataHierarchy[1].Parent)

	// Redefines
	require.Len(t, result.Redefines, 1)
	assert.Equal(t, "WS-CUST-NUM", result.Redefines[0].Item)

	// Copybook definitions
	require.Len(t, result.CopybookDefs, 1)
	assert.Equal(t, "CUSTREC", result.CopybookDefs[0].Copybook)

	// Annotations
	require.Len(t, result.Annotations, 2)
	assert.Equal(t, "0000-MAIN", result.Annotations[0].Paragraph)
	assert.Equal(t, "PROCESSING", result.Annotations[0].Category)

	// Conditional logic
	require.Len(t, result.ConditionalLogic, 1)
	assert.Equal(t, "2000-PROCESS", result.ConditionalLogic[0].Paragraph)
	assert.Equal(t, "IF", result.ConditionalLogic[0].Type)
	assert.Contains(t, result.ConditionalLogic[0].Variables, "WS-STATUS")

	// Dynamic call resolution
	require.Len(t, result.DynamicCallResolutions, 1)
	assert.Equal(t, "WS-PROG-NAME", result.DynamicCallResolutions[0].Variable)
	assert.Equal(t, []string{"CUSTRPT", "CUSTVAL"}, result.DynamicCallResolutions[0].ResolvedTargets)

	// Error handling
	require.Len(t, result.ErrorHandlers, 1)
	assert.Equal(t, "9000-ERROR", result.ErrorHandlers[0].Paragraph)
	assert.Equal(t, "FILE-STATUS", result.ErrorHandlers[0].Pattern)
}

func TestParsePass2Response_MarkdownFences(t *testing.T) {
	wrapped := "```json\n" + samplePass2JSON + "\n```"
	result, err := ParsePass2Response(wrapped, "/src/TEST.CBL", "TEST")
	require.NoError(t, err)
	assert.Len(t, result.Performs, 2)
}

func TestParsePass2Response_InvalidJSON(t *testing.T) {
	_, err := ParsePass2Response("not json", "/src/BAD.CBL", "BAD")
	assert.Error(t, err)
}
