package neo4j

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateReadOnlyQuery_AllowsReads(t *testing.T) {
	safe := []string{
		`MATCH (p:Program) RETURN p.programId`,
		`MATCH (p:Program)-[:CALLS]->(c:Program) RETURN p, c LIMIT 10`,
		`MATCH (d:DataItem {programId: $id}) RETURN d`,
		"// comment\nMATCH (n) RETURN n",
		`MATCH (n) WHERE n.name = 'CREATE_RECORD' RETURN n`, // CREATE in string literal
	}
	for _, q := range safe {
		assert.NoError(t, validateReadOnlyQuery(q), "should allow: %s", q)
	}
}

func TestValidateReadOnlyQuery_BlocksWrites(t *testing.T) {
	cases := []struct {
		query   string
		keyword string
	}{
		{`CREATE (p:Program {id: 'x'}) RETURN p`, "CREATE"},
		{`MATCH (p:Program) DELETE p`, "DELETE"},
		{`MATCH (p:Program) DETACH DELETE p`, "DETACH"},
		{`MERGE (p:Program {id: 'x'}) RETURN p`, "MERGE"},
		{`MATCH (p:Program) SET p.name = 'x'`, "SET"},
		{`MATCH (p:Program) REMOVE p.name`, "REMOVE"},
		{`DROP INDEX ON :Program(id)`, "DROP"},
		{`FOREACH (id IN ['a'] | MERGE (p:Program {id: id}))`, "FOREACH"},
		{`CALL apoc.periodic.iterate('MATCH (n) RETURN n', 'DELETE n', {})`, "CALL apoc.periodic"},
		{`CALL db.createLabel('Test')`, "CALL db.create"},
	}
	for _, tc := range cases {
		err := validateReadOnlyQuery(tc.query)
		require.Error(t, err, "should block query with %s: %s", tc.keyword, tc.query)
		assert.Contains(t, err.Error(), "not permitted")
	}
}

func TestStripStringsAndComments(t *testing.T) {
	cases := []struct {
		input    string
		contains string // should NOT appear in result
	}{
		{`MATCH (n) WHERE n.name = 'CREATE' RETURN n`, "CREATE"},
		{"// CREATE node\nMATCH (n) RETURN n", "CREATE"},
		{"/* MERGE block */\nMATCH (n) RETURN n", "MERGE"},
		{"MATCH (n) WHERE n.x = `SET` RETURN n", "SET"},
	}
	for _, tc := range cases {
		result := stripStringsAndComments(tc.input)
		assert.NotContains(t, result, tc.contains, "stripped content should not contain literal from string/comment")
	}
}

func TestLimitRegex(t *testing.T) {
	hasLimit := []string{
		"MATCH (n) RETURN n LIMIT 100",
		"MATCH (n) RETURN n LIMIT $x",
		"MATCH (n) RETURN n limit 50",
		"MATCH (n) RETURN n LIMIT 100;",
	}
	for _, q := range hasLimit {
		trimmed := q
		assert.True(t, limitRegex.MatchString(trimmed), "should detect LIMIT in: %s", q)
	}

	noLimit := []string{
		"MATCH (n) RETURN n",
		"MATCH (n) RETURN count(n)",
		"MATCH (n) RETURN n ORDER BY n.id",
	}
	for _, q := range noLimit {
		assert.False(t, limitRegex.MatchString(q), "should not detect LIMIT in: %s", q)
	}
}
