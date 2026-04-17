package neo4j

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

const (
	defaultQueryLimit = 100
	maxQueryLimit     = 1000
	queryTimeout      = 30 * time.Second
)

// QueryResult holds the output of a read-only Cypher query.
type QueryResult struct {
	Columns   []string         `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	ElapsedMs int64            `json:"elapsedMs"`
	Truncated bool             `json:"truncated"`
}

// writeKeywords are Cypher clauses forbidden in read-only queries.
var writeKeywords = []string{
	"CREATE", "DELETE", "DETACH", "MERGE", "SET", "REMOVE", "DROP", "FOREACH",
	"LOAD CSV", "CALL db.create", "CALL apoc.periodic", "CALL apoc.cypher.runmany",
	"CALL apoc.cypher.doit",
}

// limitRegex detects a trailing LIMIT clause so we don't double-append.
var limitRegex = regexp.MustCompile(`(?i)\bLIMIT\s+(\$?\w+)\s*;?\s*$`)

// RunQueryReadOnly executes an arbitrary read-only Cypher query.
// It validates the query for write keywords, auto-appends LIMIT when absent,
// enforces a 30-second timeout, and caps result rows.
func (c *Client) RunQueryReadOnly(ctx context.Context, query string, params map[string]any, limit int) (*QueryResult, error) {
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	if err := validateReadOnlyQuery(query); err != nil {
		return nil, err
	}

	trimmed := strings.TrimRight(query, " \t\n;")
	if !limitRegex.MatchString(trimmed) {
		trimmed = fmt.Sprintf("%s LIMIT %d", trimmed, limit)
	}

	if params == nil {
		params = map[string]any{}
	}

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	start := time.Now()

	session := c.NewReadOnlySession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx, trimmed, params)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}

	columns, _ := result.Keys()

	var rows []map[string]any
	truncated := false
	for result.Next(ctx) {
		if len(rows) >= limit {
			truncated = true
			break
		}
		rec := result.Record()
		row := make(map[string]any, len(rec.Keys))
		for i, k := range rec.Keys {
			row[k] = serializeNeo4jValue(rec.Values[i])
		}
		rows = append(rows, row)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("query iteration error: %w", err)
	}

	return &QueryResult{
		Columns:   columns,
		Rows:      rows,
		ElapsedMs: time.Since(start).Milliseconds(),
		Truncated: truncated,
	}, nil
}

// validateReadOnlyQuery rejects queries containing write keywords.
// String literals and comments are stripped first to avoid false positives.
func validateReadOnlyQuery(query string) error {
	cleaned := strings.ToUpper(stripStringsAndComments(query))
	for _, kw := range writeKeywords {
		upper := strings.ToUpper(kw)
		if strings.Contains(cleaned, upper) {
			return fmt.Errorf("write operation '%s' is not permitted in read-only queries", kw)
		}
	}
	return nil
}

// stripStringsAndComments removes single-quoted strings, backtick identifiers,
// and // / /* */ comments so keyword checks aren't tricked by literal content.
func stripStringsAndComments(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		switch {
		case i+1 < len(s) && s[i] == '/' && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case i+1 < len(s) && s[i] == '/' && s[i+1] == '*':
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i += 2
		case s[i] == '\'':
			i++
			for i < len(s) && s[i] != '\'' {
				if s[i] == '\\' {
					i++
				}
				i++
			}
			i++
			b.WriteByte(' ')
		case s[i] == '`':
			i++
			for i < len(s) && s[i] != '`' {
				i++
			}
			i++
			b.WriteByte(' ')
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// serializeNeo4jValue converts Neo4j driver values to JSON-friendly types.
func serializeNeo4jValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case dbtype.Node:
		m := map[string]any{"_id": val.ElementId, "_labels": val.Labels}
		for k, prop := range val.Props {
			m[k] = serializeNeo4jValue(prop)
		}
		return m
	case dbtype.Relationship:
		m := map[string]any{"_id": val.ElementId, "_type": val.Type}
		for k, prop := range val.Props {
			m[k] = serializeNeo4jValue(prop)
		}
		return m
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = serializeNeo4jValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = serializeNeo4jValue(item)
		}
		return out
	default:
		return val
	}
}
