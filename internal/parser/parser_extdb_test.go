package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExternalDBResponse_Basic(t *testing.T) {
	input := `{
		"externalTables": [
			{
				"name": "customers",
				"schema": "public",
				"columns": [
					{"name": "id", "dataType": "INTEGER", "nullable": false, "isPK": true},
					{"name": "name", "dataType": "VARCHAR(100)", "nullable": false, "isPK": false}
				]
			}
		],
		"mappings": [
			{
				"cobolTable": "CUSTOMER",
				"externalTable": "customers",
				"confidence": 0.95,
				"reason": "Name match and compatible columns",
				"columnMappings": [
					{"cobolColumn": "CUST-ID", "externalColumn": "id", "transform": "RENAMED"},
					{"cobolColumn": "CUST-NAME", "externalColumn": "name", "transform": "RENAMED"}
				]
			}
		],
		"gaps": [
			{"side": "cobol_only", "tableName": "OLD_AUDIT", "columnName": "", "description": "Not migrated"}
		],
		"dataFlows": [
			{
				"cobolProgram": "CUSTMGR",
				"operation": "SELECT",
				"db2Table": "CUSTOMER",
				"externalTable": "customers",
				"flowType": "mirror",
				"description": "Direct read mapping"
			}
		]
	}`

	result, err := ParseExternalDBResponse(input, "testdb", "postgres")
	require.NoError(t, err)

	assert.Equal(t, "testdb", result.Database.Name)
	assert.Equal(t, "postgres", result.Database.DatabaseType)

	require.Len(t, result.Tables, 1)
	assert.Equal(t, "customers", result.Tables[0].Name)
	assert.Equal(t, "public", result.Tables[0].Schema)
	require.Len(t, result.Tables[0].Columns, 2)
	assert.True(t, result.Tables[0].Columns[0].IsPK)

	require.Len(t, result.Mappings, 1)
	assert.Equal(t, "CUSTOMER", result.Mappings[0].CobolDBTable)
	assert.Equal(t, 0.95, result.Mappings[0].Confidence)
	require.Len(t, result.Mappings[0].ColumnMappings, 2)
	assert.Equal(t, "RENAMED", result.Mappings[0].ColumnMappings[0].Transform)

	require.Len(t, result.Gaps, 1)
	assert.Equal(t, "cobol_only", result.Gaps[0].Side)

	require.Len(t, result.Flows, 1)
	assert.Equal(t, "CUSTMGR", result.Flows[0].CobolProgram)
}

func TestParseExternalDBResponse_CodeBlock(t *testing.T) {
	input := "Here is the analysis:\n```json\n{\"externalTables\":[],\"mappings\":[],\"gaps\":[],\"dataFlows\":[]}\n```\nDone."

	result, err := ParseExternalDBResponse(input, "mydb", "oracle")
	require.NoError(t, err)
	assert.Equal(t, "mydb", result.Database.Name)
	assert.Equal(t, "oracle", result.Database.DatabaseType)
	assert.Empty(t, result.Tables)
}

func TestParseExternalDBResponse_NoJSON(t *testing.T) {
	_, err := ParseExternalDBResponse("no json here", "db", "pg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON found")
}

func TestParseExternalDBResponse_EmptyArrays(t *testing.T) {
	input := `{"externalTables":[],"mappings":[],"gaps":[],"dataFlows":[]}`

	result, err := ParseExternalDBResponse(input, "emptydb", "postgres")
	require.NoError(t, err)
	assert.Equal(t, "emptydb", result.Database.Name)
	assert.Empty(t, result.Tables)
	assert.Empty(t, result.Mappings)
	assert.Empty(t, result.Gaps)
	assert.Empty(t, result.Flows)
}
