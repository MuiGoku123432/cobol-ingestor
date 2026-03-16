package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepairChildOf(t *testing.T) {
	t.Run("valid JSON produces correct relationships with zero-padded FQN", func(t *testing.T) {
		jsonResp := `{
			"childOfRelationships": [
				{"child": "WS-CUST-ID", "childLevel": 5, "parent": "WS-CUSTOMER-REC", "parentLevel": 1},
				{"child": "WS-CUST-NAME", "childLevel": 5, "parent": "WS-CUSTOMER-REC", "parentLevel": 1}
			]
		}`

		rels, err := ParseRepairChildOf(jsonResp, "CUSTMAINT")
		require.NoError(t, err)
		assert.Len(t, rels, 2)

		assert.Equal(t, "CUSTMAINT.05.WS-CUST-ID", rels[0].FromKey)
		assert.Equal(t, "CUSTMAINT.01.WS-CUSTOMER-REC", rels[0].ToKey)
		assert.Equal(t, "DataItem", rels[0].FromLabel)
		assert.Equal(t, "DataItem", rels[0].ToLabel)
		assert.Equal(t, "pass5_repair", rels[0].Properties["source"])

		assert.Equal(t, "CUSTMAINT.05.WS-CUST-NAME", rels[1].FromKey)
		assert.Equal(t, "CUSTMAINT.01.WS-CUSTOMER-REC", rels[1].ToKey)
	})

	t.Run("empty child or parent entries are skipped", func(t *testing.T) {
		jsonResp := `{
			"childOfRelationships": [
				{"child": "", "childLevel": 5, "parent": "WS-REC", "parentLevel": 1},
				{"child": "WS-FIELD", "childLevel": 5, "parent": "", "parentLevel": 1},
				{"child": "WS-VALID", "childLevel": 5, "parent": "WS-REC", "parentLevel": 1}
			]
		}`

		rels, err := ParseRepairChildOf(jsonResp, "TESTPROG")
		require.NoError(t, err)
		assert.Len(t, rels, 1)
		assert.Equal(t, "TESTPROG.05.WS-VALID", rels[0].FromKey)
	})

	t.Run("names are uppercased", func(t *testing.T) {
		jsonResp := `{
			"childOfRelationships": [
				{"child": "ws-field", "childLevel": 5, "parent": "ws-record", "parentLevel": 1}
			]
		}`

		rels, err := ParseRepairChildOf(jsonResp, "PROG1")
		require.NoError(t, err)
		require.Len(t, rels, 1)
		assert.Equal(t, "PROG1.05.WS-FIELD", rels[0].FromKey)
		assert.Equal(t, "PROG1.01.WS-RECORD", rels[0].ToKey)
	})

	t.Run("markdown fences are stripped", func(t *testing.T) {
		jsonResp := "```json\n{\"childOfRelationships\": [{\"child\": \"WS-A\", \"childLevel\": 5, \"parent\": \"WS-B\", \"parentLevel\": 1}]}\n```"

		rels, err := ParseRepairChildOf(jsonResp, "PROG2")
		require.NoError(t, err)
		assert.Len(t, rels, 1)
	})
}
