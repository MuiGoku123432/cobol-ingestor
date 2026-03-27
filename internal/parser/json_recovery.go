package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// reTrailingComma matches a comma followed by optional whitespace and a closing brace or bracket.
var reTrailingComma = regexp.MustCompile(`,\s*([}\]])`)

// reJSONStructChar matches any JSON structural character: comma, brace, bracket, quote, or colon.
// Used to distinguish prose trailing text from a truncated JSON tail.
var reJSONStructChar = regexp.MustCompile(`[,{}\[\]":]`)

// sanitizeJSON cleans up common LLM response artifacts before attempting JSON parsing.
// It strips leading/trailing non-JSON text, removes trailing commas, and removes
// JavaScript-style comments (// and /* */) outside of string values.
// Single-quoted strings are intentionally not handled to avoid corrupting COBOL
// content that may contain apostrophes.
func sanitizeJSON(raw string) string {
	// Step 1: Strip leading non-JSON text — find the first '{'.
	if idx := strings.Index(raw, "{"); idx > 0 {
		raw = raw[idx:]
	}

	// Step 2: Strip trailing non-JSON text — find the last '}' and trim everything
	// after it, but only when the trailing content looks like prose rather than the
	// tail of a truncated JSON structure. We check that the tail contains no JSON
	// structural characters (commas, braces, brackets, quotes, or colons), which
	// distinguishes "Hope this helps!" from ", {"from": "C"".
	if idx := strings.LastIndex(raw, "}"); idx >= 0 && idx < len(raw)-1 {
		tail := strings.TrimSpace(raw[idx+1:])
		if len(tail) > 0 && !reJSONStructChar.MatchString(tail) {
			raw = raw[:idx+1]
		}
	}

	// Step 3: Remove // single-line comments and /* */ block comments outside strings.
	raw = removeComments(raw)

	// Step 4: Remove trailing commas before } or ].
	raw = reTrailingComma.ReplaceAllString(raw, "$1")

	return raw
}

// removeComments strips // single-line and /* */ block comments from s,
// being careful not to modify content inside double-quoted JSON strings.
func removeComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	i := 0
	for i < len(s) {
		// Handle double-quoted strings: copy verbatim until closing quote.
		if s[i] == '"' {
			b.WriteByte(s[i])
			i++
			for i < len(s) {
				c := s[i]
				b.WriteByte(c)
				if c == '\\' && i+1 < len(s) {
					// Escaped character — copy the next byte as-is.
					i++
					b.WriteByte(s[i])
				} else if c == '"' {
					// End of string.
					break
				}
				i++
			}
			i++
			continue
		}

		// Check for // single-line comment.
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			// Skip until end of line.
			i += 2
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}

		// Check for /* block comment.
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			// Skip until closing */.
			i += 2
			for i+1 < len(s) {
				if s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		b.WriteByte(s[i])
		i++
	}

	return b.String()
}

// isValidJSON attempts to unmarshal s into a map[string]any and returns true if successful.
func isValidJSON(s string) bool {
	var m map[string]any
	return json.Unmarshal([]byte(s), &m) == nil
}

// RecoverPartialJSON attempts to recover valid JSON from a truncated LLM response.
// It handles common truncation points: mid-string, mid-array, mid-object.
// Returns the recovered JSON string or an error if recovery is not possible.
func RecoverPartialJSON(raw string) (string, error) {
	// Step 1: Sanitize — strip preamble/postamble text and remove comments/trailing commas.
	raw = sanitizeJSON(raw)

	// Step 2: If it's already valid JSON, return as-is.
	if isValidJSON(raw) {
		return raw, nil
	}

	// Step 3: Parse to find the structure — track brace/bracket depth and string state.
	type delimInfo struct {
		char byte // '{' or '['
	}

	// scan returns the stack of open delimiters and the position of the last valid
	// array element boundary (a ',' or ']' at the correct depth).
	scan := func(s string) (stack []delimInfo, lastBoundary int) {
		var inString bool
		var escaped bool
		stack = make([]delimInfo, 0, 32)
		lastBoundary = -1

		for i := 0; i < len(s); i++ {
			c := s[i]

			if escaped {
				escaped = false
				continue
			}

			if inString {
				if c == '\\' {
					escaped = true
				} else if c == '"' {
					inString = false
				}
				continue
			}

			switch c {
			case '"':
				inString = true
			case '{':
				stack = append(stack, delimInfo{char: '{'})
			case '[':
				stack = append(stack, delimInfo{char: '['})
			case '}':
				if len(stack) > 0 && stack[len(stack)-1].char == '{' {
					stack = stack[:len(stack)-1]
				}
			case ']':
				if len(stack) > 0 && stack[len(stack)-1].char == '[' {
					stack = stack[:len(stack)-1]
				}
				// A ']' that closes an array is a valid boundary.
				lastBoundary = i + 1
			case ',':
				// A comma between elements is a valid truncation point.
				lastBoundary = i
			}
		}
		return stack, lastBoundary
	}

	// closeDelimiters appends the matching closing delimiter for each open one, in reverse.
	closeDelimiters := func(s string, stack []delimInfo) string {
		// Strip trailing commas before closing.
		s = strings.TrimRight(s, " \t\n\r")
		s = strings.TrimRight(s, ",")
		s = strings.TrimRight(s, " \t\n\r")

		var b strings.Builder
		b.WriteString(s)
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].char == '{' {
				b.WriteByte('}')
			} else {
				b.WriteByte(']')
			}
		}
		return b.String()
	}

	// Step 4: Find the last valid array element boundary and try truncating there.
	stack, lastBoundary := scan(raw)

	if lastBoundary > 0 {
		truncated := raw[:lastBoundary]
		// Re-scan the truncated portion to get an accurate stack.
		truncStack, _ := scan(truncated)
		candidate := closeDelimiters(truncated, truncStack)
		if isValidJSON(candidate) {
			return candidate, nil
		}
	}

	// Step 5: If that didn't work, try closing from the current end.
	candidate := closeDelimiters(raw, stack)
	if isValidJSON(candidate) {
		return candidate, nil
	}

	// Step 6: Progressively remove trailing array elements — find the last ','
	// before the current end, truncate there, close brackets/braces, and retry.
	// Try up to 10 times.
	current := raw
	if lastBoundary > 0 && lastBoundary < len(current) {
		current = current[:lastBoundary]
	}

	for attempt := 0; attempt < 10; attempt++ {
		// Find the last comma in the current string (outside of strings).
		var inStr, esc bool
		lastComma := -1
		for i := 0; i < len(current); i++ {
			c := current[i]
			if esc {
				esc = false
				continue
			}
			if inStr {
				if c == '\\' {
					esc = true
				} else if c == '"' {
					inStr = false
				}
				continue
			}
			switch c {
			case '"':
				inStr = true
			case ',':
				lastComma = i
			}
		}

		if lastComma <= 0 {
			break
		}

		current = current[:lastComma]
		curStack, _ := scan(current)
		candidate = closeDelimiters(current, curStack)
		if isValidJSON(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to recover valid JSON from truncated response (%d bytes)", len(raw))
}
