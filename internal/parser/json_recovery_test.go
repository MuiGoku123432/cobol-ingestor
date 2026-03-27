package parser

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoverPartialJSON_ValidJSONPassesThrough(t *testing.T) {
	input := `{"programId": "CUSTMAINT", "copyReferences": ["CUSTFILE", "ERRHAND"]}`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)
	assert.Equal(t, input, result, "valid JSON should be returned unchanged")

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m))
	assert.Equal(t, "CUSTMAINT", m["programId"])
	refs := m["copyReferences"].([]any)
	assert.Len(t, refs, 2)
}

func TestRecoverPartialJSON_TruncatedMidArray(t *testing.T) {
	// Truncated while writing the second array element — P1 should survive.
	input := `{"programs": [{"id": "P1"}, {"id": "P2"`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	programs, ok := m["programs"].([]any)
	require.True(t, ok, "programs should be an array")
	require.GreaterOrEqual(t, len(programs), 1, "at least one complete program should survive")

	first := programs[0].(map[string]any)
	assert.Equal(t, "P1", first["id"])
}

func TestRecoverPartialJSON_TruncatedMidString(t *testing.T) {
	// Truncated in the middle of a string value with no prior complete elements.
	// The recovery function relies on comma/bracket boundaries to find truncation
	// points, so a single field truncated mid-string with no prior boundary
	// cannot be recovered.
	input := `{"programId": "CUST`

	_, err := RecoverPartialJSON(input)
	assert.Error(t, err, "mid-string truncation with no prior boundary should fail")
}

func TestRecoverPartialJSON_TruncatedMidStringWithPriorFields(t *testing.T) {
	// When there is a prior complete field before the mid-string truncation,
	// recovery should succeed by truncating back to the last comma boundary.
	input := `{"programId": "CUSTMAINT", "executionMode": "BAT`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "CUSTMAINT", m["programId"])
}

func TestRecoverPartialJSON_TruncatedMidObject(t *testing.T) {
	// Two array elements, second is incomplete (missing "to" value).
	// First complete element should survive.
	input := `{"performs": [{"from": "A", "to": "B"}, {"from": "C"`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	performs, ok := m["performs"].([]any)
	require.True(t, ok, "performs should be an array")
	require.GreaterOrEqual(t, len(performs), 1, "at least one complete perform should survive")

	first := performs[0].(map[string]any)
	assert.Equal(t, "A", first["from"])
	assert.Equal(t, "B", first["to"])
}

func TestRecoverPartialJSON_MultipleNestedArrays(t *testing.T) {
	// Deeply nested structure with truncation inside inner array.
	input := `{"level1": {"level2": [{"items": [{"name": "A"}, {"name": "B"}]}, {"items": [{"name": "C"`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	// Should at least have the outer structure
	level1, ok := m["level1"].(map[string]any)
	require.True(t, ok, "level1 should be a map")

	level2, ok := level1["level2"].([]any)
	require.True(t, ok, "level2 should be an array")
	require.GreaterOrEqual(t, len(level2), 1, "at least one nested element should survive")

	// First element should have its full items array
	firstElem := level2[0].(map[string]any)
	items := firstElem["items"].([]any)
	assert.Len(t, items, 2, "first element should have both items")
}

func TestRecoverPartialJSON_EmptyArraysPreserved(t *testing.T) {
	// First array is empty (valid), second is truncated.
	input := `{"items": [], "other": [{"x": 1`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	items, ok := m["items"].([]any)
	require.True(t, ok, "items should be an array")
	assert.Empty(t, items, "empty items array should be preserved")
}

func TestRecoverPartialJSON_EscapedQuotesInStrings(t *testing.T) {
	// String contains escaped quotes, then truncation occurs.
	// With no prior comma boundary, this single-field case cannot be recovered.
	input := `{"text": "he said \"hello`

	_, err := RecoverPartialJSON(input)
	assert.Error(t, err, "mid-string truncation with escaped quotes and no prior boundary should fail")
}

func TestRecoverPartialJSON_EscapedQuotesWithPriorField(t *testing.T) {
	// String with escaped quotes, but there is a prior complete field so
	// recovery can truncate back to the comma boundary.
	input := `{"id": "P1", "text": "he said \"hello`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "P1", m["id"])
}

func TestRecoverPartialJSON_SingleCompleteElement(t *testing.T) {
	// Array with one complete element and a second that is barely started.
	input := `{"callTargets": [{"target": "CUSTRPT", "isDynamic": false}, {"`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	targets, ok := m["callTargets"].([]any)
	require.True(t, ok, "callTargets should be an array")
	require.GreaterOrEqual(t, len(targets), 1, "at least one call target should survive")

	first := targets[0].(map[string]any)
	assert.Equal(t, "CUSTRPT", first["target"])
	assert.Equal(t, false, first["isDynamic"])
}

func TestRecoverPartialJSON_CompletelyInvalid(t *testing.T) {
	input := "not json at all"

	_, err := RecoverPartialJSON(input)
	assert.Error(t, err, "completely invalid input should return an error")
}

func TestRecoverPartialJSON_ValidWithTrailingGarbage(t *testing.T) {
	// Valid JSON followed by trailing characters (e.g., extra text from LLM).
	input := `{"programId": "TEST"}some trailing garbage`

	result, err := RecoverPartialJSON(input)
	// The function may or may not recover this; if it does, verify valid JSON.
	if err == nil {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	}
	// If it errors, that is also acceptable for malformed input.
}

func TestRecoverPartialJSON_TruncatedAfterComma(t *testing.T) {
	// Truncation right after a comma separating array elements.
	input := `{"paragraphs": ["MAIN-PARA", "INIT-PARA",`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	paras, ok := m["paragraphs"].([]any)
	require.True(t, ok, "paragraphs should be an array")
	require.GreaterOrEqual(t, len(paras), 2, "both complete paragraphs should survive")
	assert.Equal(t, "MAIN-PARA", paras[0])
	assert.Equal(t, "INIT-PARA", paras[1])
}

func TestRecoverPartialJSON_TruncatedBetweenFields(t *testing.T) {
	// Truncation after a complete field, before the next key starts.
	input := `{"programId": "TEST", "copyReferences": ["A", "B"],`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	assert.Equal(t, "TEST", m["programId"])
	refs := m["copyReferences"].([]any)
	assert.Len(t, refs, 2)
}

func TestRecoverPartialJSON_LargeRealisticTruncation(t *testing.T) {
	// Simulates a realistic LLM response that got cut off mid-way.
	input := `{
  "programId": "CUSTMAINT",
  "executionMode": "BATCH",
  "copyReferences": ["CUSTCOPY", "ERRCOPY"],
  "callTargets": [
    {"target": "CUSTRPT", "isDynamic": false},
    {"target": "CUSTVAL", "isDynamic": true}
  ],
  "paragraphs": ["0000-MAIN", "1000-INIT", "2000-PROCESS"],
  "sections": ["MAIN-LOGIC"],
  "fileDefinitions": [
    {"name": "CUSTOMER-FILE", "organization": "INDEXED", "vsamType": "KSDS", "dataStoreType": "VSAM"}
  ],
  "dataItems": [
    {"name": "WS-CUSTOMER-REC", "level": 1, "picture": "", "usage": ""},
    {"name": "WS-RETURN-CODE", "level": 77, "picture": "S9(4)`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	assert.Equal(t, "CUSTMAINT", m["programId"])
	assert.Equal(t, "BATCH", m["executionMode"])

	// copyReferences should be fully intact
	refs := m["copyReferences"].([]any)
	assert.Len(t, refs, 2)

	// callTargets should be fully intact
	targets := m["callTargets"].([]any)
	assert.Len(t, targets, 2)

	// paragraphs should be fully intact
	paras := m["paragraphs"].([]any)
	assert.Len(t, paras, 3)
}

func TestRecoverPartialJSON_EmptyObject(t *testing.T) {
	// A valid empty object should pass through.
	input := `{}`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)
	assert.Equal(t, input, result)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m))
	assert.Empty(t, m)
}

func TestRecoverPartialJSON_TruncatedAtColon(t *testing.T) {
	// Truncation right after a key's colon, before the value.
	input := `{"programId":`

	result, err := RecoverPartialJSON(input)
	// This is a borderline case; either recovery or error is acceptable.
	if err == nil {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	}
}

// --- sanitizeJSON / RecoverPartialJSON sanitization tests ---

func TestSanitizeJSON_LeadingText(t *testing.T) {
	// LLM prefaces the JSON with natural-language text.
	input := "Here is the JSON:\n{\"programId\": \"TEST\"}"

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "TEST", m["programId"])
}

func TestSanitizeJSON_TrailingText(t *testing.T) {
	// LLM appends a closing remark after valid JSON.
	input := "{\"programId\": \"TEST\"}\nHope this helps!"

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "TEST", m["programId"])
}

func TestSanitizeJSON_TrailingCommas(t *testing.T) {
	// Trailing commas before } and ] are illegal in JSON but common in LLM output.
	input := `{"items": ["a", "b",], "x": 1,}`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	items, ok := m["items"].([]any)
	require.True(t, ok, "items should be an array")
	assert.Len(t, items, 2)
	assert.Equal(t, float64(1), m["x"])
}

func TestSanitizeJSON_LineComments(t *testing.T) {
	// Single-line // comments are stripped outside of string values.
	input := "{\"key\": \"value\" // comment\n}"

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "value", m["key"])
}

func TestSanitizeJSON_BlockComments(t *testing.T) {
	// Block /* ... */ comments are stripped outside of string values.
	input := `{"key": /* inline */ "value"}`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "value", m["key"])
}

func TestSanitizeJSON_Mixed(t *testing.T) {
	// Leading preamble text, trailing commas, and line comments all at once.
	input := "Here is the output:\n{\"id\": \"P1\", // program id\n\"refs\": [\"A\", \"B\",],}"

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "P1", m["id"])

	refs, ok := m["refs"].([]any)
	require.True(t, ok, "refs should be an array")
	assert.Len(t, refs, 2)
}

func TestSanitizeJSON_StringsWithSlashSlash(t *testing.T) {
	// A string value containing // must not be treated as a comment.
	input := `{"url": "http://example.com"}`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "http://example.com", m["url"])
}

func TestSanitizeJSON_StringsWithBraces(t *testing.T) {
	// A string value containing { and } must not confuse the leading/trailing strip.
	input := `{"template": "use {placeholder} here", "id": "T1"}`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)
	assert.Equal(t, "use {placeholder} here", m["template"])
	assert.Equal(t, "T1", m["id"])
}

func TestRecoverPartialJSON_NestedObjectsTruncated(t *testing.T) {
	// Object containing another object, truncated inside the inner one.
	input := `{"config": {"host": "localhost", "port": 8080}, "data": {"name": "test", "val`

	result, err := RecoverPartialJSON(input)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &m), "recovered JSON should be valid: %s", result)

	// The complete config object should survive.
	config, ok := m["config"].(map[string]any)
	require.True(t, ok, "config should be a map")
	assert.Equal(t, "localhost", config["host"])
	assert.Equal(t, float64(8080), config["port"])
}
