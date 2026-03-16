package chunker

import (
	"path/filepath"
	"strings"
)

// BWChunk represents a chunk of a Businessware file ready for analysis.
type BWChunk struct {
	FileName string
	Content  string
	Index    int
	Total    int
}

// ChunkBWFile splits a Businessware file into chunks respecting the token limit.
// Java files are split at class/method boundaries, markdown at heading boundaries,
// and everything else at blank-line paragraph boundaries.
func ChunkBWFile(path string, content []byte, tokenLimit int) ([]BWChunk, error) {
	text := string(content)
	tokens := EstimateTokens(text)

	if tokens <= tokenLimit {
		return []BWChunk{{
			FileName: path,
			Content:  text,
			Index:    0,
			Total:    1,
		}}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	var sections []string

	switch ext {
	case ".java":
		sections = splitJava(text)
	case ".md":
		sections = splitMarkdown(text)
	default:
		sections = splitBlankLines(text)
	}

	return groupSections(path, sections, tokenLimit), nil
}

// splitJava splits Java source at class/method boundaries.
func splitJava(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Split at class, interface, enum declarations or method-like signatures
		isClassDecl := (strings.Contains(trimmed, "class ") ||
			strings.Contains(trimmed, "interface ") ||
			strings.Contains(trimmed, "enum ")) &&
			!strings.HasPrefix(trimmed, "//") &&
			!strings.HasPrefix(trimmed, "*")

		isMethodLike := len(trimmed) > 0 &&
			strings.Contains(trimmed, "(") &&
			(strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, ")")) &&
			!strings.HasPrefix(trimmed, "//") &&
			!strings.HasPrefix(trimmed, "*") &&
			!strings.HasPrefix(trimmed, "if") &&
			!strings.HasPrefix(trimmed, "for") &&
			!strings.HasPrefix(trimmed, "while")

		if (isClassDecl || isMethodLike) && len(current) > 0 {
			sections = append(sections, strings.Join(current, "\n"))
			current = nil
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	return sections
}

// splitMarkdown splits markdown content at heading boundaries.
func splitMarkdown(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") && len(current) > 0 {
			sections = append(sections, strings.Join(current, "\n"))
			current = nil
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	return sections
}

// splitBlankLines splits content at blank-line paragraph boundaries.
func splitBlankLines(content string) []string {
	paragraphs := strings.Split(content, "\n\n")
	var sections []string
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			sections = append(sections, p)
		}
	}
	if len(sections) == 0 {
		sections = []string{content}
	}
	return sections
}

// groupSections groups sections into chunks that fit within the token limit.
func groupSections(path string, sections []string, tokenLimit int) []BWChunk {
	var chunks []BWChunk
	var current []string
	currentTokens := 0

	for _, sec := range sections {
		secTokens := EstimateTokens(sec)

		if currentTokens+secTokens > tokenLimit && len(current) > 0 {
			chunks = append(chunks, BWChunk{
				FileName: path,
				Content:  strings.Join(current, "\n"),
			})
			current = nil
			currentTokens = 0
		}

		current = append(current, sec)
		currentTokens += secTokens
	}
	if len(current) > 0 {
		chunks = append(chunks, BWChunk{
			FileName: path,
			Content:  strings.Join(current, "\n"),
		})
	}

	// Set Index/Total
	for i := range chunks {
		chunks[i].Index = i
		chunks[i].Total = len(chunks)
	}

	return chunks
}
