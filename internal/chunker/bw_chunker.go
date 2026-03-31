package chunker

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"cobol-ingestor/internal/diagram"
)

// Pre-compiled regexes for content shortening.
var (
	javaBlockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	javaLineCommentRe  = regexp.MustCompile(`(?m)//[^\n]*`)
	javaImportRe       = regexp.MustCompile(`(?m)^import\s+(static\s+)?([a-zA-Z0-9_.]+)\.[A-Za-z0-9_*]+;\s*$`)
	xmlCommentRe       = regexp.MustCompile(`(?s)<!--.*?-->`)
	blankLinesRe       = regexp.MustCompile(`\n{3,}`)
	javaPackageRe      = regexp.MustCompile(`(?m)^package\s+[^;]+;`)
	javaClassDeclRe    = regexp.MustCompile(`(?m)^(public\s+)?(abstract\s+)?(final\s+)?(class|interface|enum)\s+\S+[^{]*\{`)
	javaFieldRe        = regexp.MustCompile(`(?m)^\s+(private|protected|public)\s+(static\s+)?(final\s+)?[\w<>\[\],\s]+\s+\w+\s*[=;]`)
	javaMethodSigRe    = regexp.MustCompile(`(?m)^\s+(public|private|protected)\s+(static\s+)?([\w<>\[\]]+)\s+(\w+)\s*\([^)]*\)`)
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
// Content is shortened before chunking to reduce token consumption.
func ChunkBWFile(path string, content []byte, tokenLimit int) ([]BWChunk, error) {
	ext := strings.ToLower(filepath.Ext(path))

	// 1. Pre-process diagram formats: extract text from binary/compressed content
	var text string
	if diagram.IsDiagramExtension(ext) {
		extracted, err := diagram.ExtractText(path, content)
		if err != nil {
			return nil, fmt.Errorf("extracting diagram %s: %w", path, err)
		}
		text = extracted
	} else {
		text = string(content)
	}

	// 2. Apply file-type-specific shortening
	switch ext {
	case ".java", ".class":
		text = shortenJava(text)
	case ".xml":
		text = shortenXML(text)
	}

	// 3. Apply general shortening
	text = trimTrailingWhitespace(text)
	text = collapseBlankLines(text)

	// 4. Reserve space for prompt overhead
	effectiveLimit := tokenLimit - PromptOverheadTokens
	if effectiveLimit < 500 {
		effectiveLimit = 500
	}

	// 5. Token estimation
	tokens := EstimateTokens(text)

	if tokens <= effectiveLimit {
		return []BWChunk{{
			FileName: path,
			Content:  text,
			Index:    0,
			Total:    1,
		}}, nil
	}

	// 6. Split into sections
	var sections []string

	switch ext {
	case ".java", ".class":
		sections = splitJava(text)
	case ".md":
		sections = splitMarkdown(text)
	case ".properties", ".json", ".yml", ".yaml":
		sections = splitBlankLines(text)
	default:
		sections = splitBlankLines(text)
	}

	// 7. Group with preamble for Java multi-chunk files
	return groupSectionsWithPreamble(path, ext, text, sections, effectiveLimit), nil
}

// collapseBlankLines replaces 3+ consecutive newlines with 2 (one blank line).
func collapseBlankLines(content string) string {
	return blankLinesRe.ReplaceAllString(content, "\n\n")
}

// trimTrailingWhitespace removes trailing spaces and tabs from each line.
func trimTrailingWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// shortenJava applies Java-specific shortening: strips comments and summarizes imports.
func shortenJava(content string) string {
	// Strip block comments (including Javadoc)
	content = javaBlockCommentRe.ReplaceAllString(content, "")

	// Strip line comments
	content = javaLineCommentRe.ReplaceAllString(content, "")

	// Summarize imports
	content = summarizeImports(content)

	return content
}

// summarizeImports replaces individual import statements with a compact summary.
func summarizeImports(content string) string {
	matches := javaImportRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content
	}

	regularPkgs := make(map[string]bool)
	staticPkgs := make(map[string]bool)

	for _, m := range matches {
		isStatic := strings.TrimSpace(m[1]) == "static"
		pkg := m[2]
		if isStatic {
			staticPkgs[pkg] = true
		} else {
			regularPkgs[pkg] = true
		}
	}

	// Remove all import lines
	content = javaImportRe.ReplaceAllString(content, "")

	// Build summary
	var summary strings.Builder
	if len(regularPkgs) > 0 {
		pkgs := sortedKeys(regularPkgs)
		summary.WriteString("// Imports: ")
		summary.WriteString(strings.Join(pkgs, ", "))
		summary.WriteString("\n")
	}
	if len(staticPkgs) > 0 {
		pkgs := sortedKeys(staticPkgs)
		summary.WriteString("// Static imports: ")
		summary.WriteString(strings.Join(pkgs, ", "))
		summary.WriteString("\n")
	}

	// Insert summary after package declaration (or at start if none)
	pkgMatch := javaPackageRe.FindStringIndex(content)
	if pkgMatch != nil {
		// Find end of line after package declaration
		afterPkg := pkgMatch[1]
		nlIdx := strings.Index(content[afterPkg:], "\n")
		if nlIdx >= 0 {
			insertAt := afterPkg + nlIdx + 1
			content = content[:insertAt] + summary.String() + content[insertAt:]
		} else {
			content = content + "\n" + summary.String()
		}
	} else {
		content = summary.String() + content
	}

	return content
}

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple insertion sort for typically small sets
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// shortenXML applies XML-specific shortening: strips XML comments.
func shortenXML(content string) string {
	return xmlCommentRe.ReplaceAllString(content, "")
}

// summarizeJavaPreamble produces a compact skeleton of a Java class for use
// as context in subsequent chunks of a multi-chunk file.
func summarizeJavaPreamble(content string) string {
	var sb strings.Builder
	sb.WriteString("// >> PREAMBLE SUMMARY (full class omitted to reduce token count)\n")

	// Package declaration
	if pkg := javaPackageRe.FindString(content); pkg != "" {
		sb.WriteString(pkg)
		sb.WriteString("\n")
	}

	// Import summary (already summarized, just find the comment lines)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// Imports:") || strings.HasPrefix(trimmed, "// Static imports:") {
			sb.WriteString(trimmed)
			sb.WriteString("\n")
		}
	}

	// Class/interface/enum declaration
	if classDecl := javaClassDeclRe.FindString(content); classDecl != "" {
		sb.WriteString(strings.TrimSpace(classDecl))
		sb.WriteString("\n")
	}

	// Field declarations
	fields := javaFieldRe.FindAllString(content, -1)
	if len(fields) > 0 {
		sb.WriteString("  // Fields: ")
		var fieldSummaries []string
		for _, f := range fields {
			f = strings.TrimSpace(f)
			// Remove trailing = or ; for cleaner display
			f = strings.TrimRight(f, " =;")
			fieldSummaries = append(fieldSummaries, f)
		}
		sb.WriteString(strings.Join(fieldSummaries, ", "))
		sb.WriteString("\n")
	}

	// Method signatures
	methods := javaMethodSigRe.FindAllStringSubmatch(content, -1)
	if len(methods) > 0 {
		sb.WriteString("  // Methods: ")
		var methodSummaries []string
		for _, m := range methods {
			// m[3] = return type, m[4] = method name
			methodSummaries = append(methodSummaries, m[3]+" "+m[4]+"()")
		}
		sb.WriteString(strings.Join(methodSummaries, ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// groupSectionsWithPreamble wraps groupSections with Java preamble injection
// for multi-chunk files. Non-Java files delegate directly to groupSections.
func groupSectionsWithPreamble(path, ext, fullContent string, sections []string, tokenLimit int) []BWChunk {
	chunks := groupSections(path, sections, tokenLimit)

	if (ext != ".java" && ext != ".class") || len(chunks) <= 1 {
		return chunks
	}

	// Generate preamble for subsequent chunks
	preamble := summarizeJavaPreamble(fullContent)
	preambleTokens := EstimateTokens(preamble)

	// Re-group with reduced budget for chunks 1..N to account for preamble
	// Chunk 0 uses the full content, chunks 1..N get preamble prefixed
	reducedLimit := tokenLimit - preambleTokens
	if reducedLimit < 500 {
		// Preamble is too large relative to limit; skip preamble injection
		return chunks
	}

	// Re-chunk: first chunk at full budget, subsequent chunks at reduced budget
	var result []BWChunk
	var current []string
	currentTokens := 0
	chunkIdx := 0

	budget := tokenLimit // first chunk gets full budget

	for _, sec := range sections {
		secTokens := EstimateTokens(sec)

		if currentTokens+secTokens > budget && len(current) > 0 {
			chunkContent := strings.Join(current, "\n")
			if chunkIdx > 0 {
				chunkContent = preamble + chunkContent
			}
			result = append(result, BWChunk{
				FileName: path,
				Content:  chunkContent,
			})
			current = nil
			currentTokens = 0
			chunkIdx++
			budget = reducedLimit // subsequent chunks have reduced budget
		}

		current = append(current, sec)
		currentTokens += secTokens
	}
	if len(current) > 0 {
		chunkContent := strings.Join(current, "\n")
		if chunkIdx > 0 {
			chunkContent = preamble + chunkContent
		}
		result = append(result, BWChunk{
			FileName: path,
			Content:  chunkContent,
		})
	}

	// Set Index/Total
	for i := range result {
		result[i].Index = i
		result[i].Total = len(result)
	}

	return result
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
