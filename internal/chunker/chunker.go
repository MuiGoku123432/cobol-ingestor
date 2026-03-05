package chunker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// Chunk represents a piece of a source file ready for analysis.
type Chunk struct {
	FileName string
	Content  string
	FileInfo graph.FileInfo
	Index    int // chunk index within the file (0 for single-chunk)
	Total    int // total chunks for this file
	Pass     int // 1 or 2
}

// ChunkFile reads a file and returns chunks suitable for Pass 1 analysis.
// For Pass 1, files are sent as a single chunk (no splitting needed).
func ChunkFile(fi graph.FileInfo, tokenLimit int, logger *zap.Logger) ([]Chunk, error) {
	data, err := os.ReadFile(fi.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fi.Path, err)
	}

	content := string(data)
	tokens := EstimateTokens(content)

	if tokens > tokenLimit {
		logger.Warn("file exceeds token limit, truncating",
			zap.String("file", fi.Path),
			zap.Int("estimated_tokens", tokens),
			zap.Int("limit", tokenLimit),
		)
		// Truncate to fit within limit (rough: 4 chars per token)
		maxChars := tokenLimit * 4
		if maxChars < len(content) {
			content = content[:maxChars]
		}
	}

	return []Chunk{
		{
			FileName: fi.Path,
			Content:  content,
			FileInfo: fi,
			Index:    0,
			Total:    1,
			Pass:     1,
		},
	}, nil
}

// EstimateTokens provides a rough token count (chars / 4).
func EstimateTokens(content string) int {
	return len(content) / 4
}

// CopybookIndex maps normalized copybook names to file paths.
type CopybookIndex map[string]string

// BuildCopybookIndex creates a map from uppercase stem to full path
// for all copybook files from scanner results.
func BuildCopybookIndex(files []graph.FileInfo) CopybookIndex {
	idx := make(CopybookIndex)
	for _, f := range files {
		if f.Type != graph.FileTypeCopybook {
			continue
		}
		stem := strings.ToUpper(strings.TrimSuffix(filepath.Base(f.Path), filepath.Ext(f.Path)))
		idx[stem] = f.Path
	}
	return idx
}

var copyRegex = regexp.MustCompile(`(?im)^\s{6}\s+COPY\s+([A-Za-z0-9-]+)`)

// InlineCopybooks replaces COPY statements with copybook content.
// Tracks visited set to prevent circular references. maxDepth prevents runaway recursion.
func InlineCopybooks(content string, index CopybookIndex, maxDepth int) (string, error) {
	return inlineCopybooksRecurse(content, index, maxDepth, make(map[string]bool))
}

func inlineCopybooksRecurse(content string, index CopybookIndex, depth int, visited map[string]bool) (string, error) {
	if depth <= 0 {
		return content, nil
	}

	return copyRegex.ReplaceAllStringFunc(content, func(match string) string {
		subs := copyRegex.FindStringSubmatch(match)
		if len(subs) < 2 {
			return match
		}
		name := strings.ToUpper(subs[1])

		if visited[name] {
			return match // circular reference, leave as-is
		}

		path, ok := index[name]
		if !ok {
			return match // copybook not found, leave as-is
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return match
		}

		visited[name] = true
		inlined, _ := inlineCopybooksRecurse(string(data), index, depth-1, visited)
		delete(visited, name) // allow same copybook in different branches

		return fmt.Sprintf("      *>> COPY %s INLINED BEGIN\n%s\n      *>> COPY %s INLINED END", name, inlined, name)
	}), nil
}

// Pass2ChunkOptions configures Pass 2 chunking behavior.
type Pass2ChunkOptions struct {
	TokenLimit    int
	OverlapLines  int // lines of overlap between chunks (default 20)
	CopybookIndex CopybookIndex
}

// ChunkFilePass2 reads a file, inlines copybooks, splits divisions, and chunks
// the PROCEDURE DIVISION at paragraph boundaries.
func ChunkFilePass2(fi graph.FileInfo, opts Pass2ChunkOptions, logger *zap.Logger) ([]Chunk, error) {
	data, err := os.ReadFile(fi.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fi.Path, err)
	}

	content := string(data)

	// Inline copybooks
	if opts.CopybookIndex != nil {
		content, err = InlineCopybooks(content, opts.CopybookIndex, 10)
		if err != nil {
			return nil, fmt.Errorf("inlining copybooks for %s: %w", fi.Path, err)
		}
	}

	// Split into divisions
	divs := splitDivisions(content)

	// Build preamble from non-PROCEDURE divisions
	preamble := ""
	for _, name := range []string{"IDENTIFICATION", "ENVIRONMENT", "DATA"} {
		if div, ok := divs[name]; ok {
			preamble += div + "\n"
		}
	}

	procedure := divs["PROCEDURE"]
	if procedure == "" {
		// No procedure division — return whole file as single chunk
		return []Chunk{{
			FileName: fi.Path,
			Content:  content,
			FileInfo: fi,
			Index:    0,
			Total:    1,
			Pass:     2,
		}}, nil
	}

	preambleTokens := EstimateTokens(preamble)
	procedureTokens := EstimateTokens(procedure)
	budget := opts.TokenLimit - preambleTokens

	if budget <= 0 {
		logger.Warn("preamble alone exceeds token limit",
			zap.String("file", fi.Path),
			zap.Int("preamble_tokens", preambleTokens),
		)
		budget = opts.TokenLimit / 2
	}

	// If everything fits in one chunk, return it
	if preambleTokens+procedureTokens <= opts.TokenLimit {
		return []Chunk{{
			FileName: fi.Path,
			Content:  preamble + procedure,
			FileInfo: fi,
			Index:    0,
			Total:    1,
			Pass:     2,
		}}, nil
	}

	// Split PROCEDURE at paragraph boundaries and group greedily
	paragraphs := splitParagraphs(procedure)
	var chunks []Chunk
	var currentLines []string
	currentTokens := 0

	overlapLines := opts.OverlapLines
	if overlapLines <= 0 {
		overlapLines = 20
	}

	var overlapBuffer []string

	for _, para := range paragraphs {
		paraTokens := EstimateTokens(para.content)

		if currentTokens+paraTokens > budget && len(currentLines) > 0 {
			// Emit current chunk
			chunks = append(chunks, Chunk{
				FileName: fi.Path,
				Content:  preamble + strings.Join(currentLines, "\n"),
				FileInfo: fi,
				Pass:     2,
			})

			// Start new chunk with overlap from end of previous
			currentLines = make([]string, len(overlapBuffer))
			copy(currentLines, overlapBuffer)
			currentTokens = EstimateTokens(strings.Join(currentLines, "\n"))
		}

		lines := strings.Split(para.content, "\n")
		currentLines = append(currentLines, lines...)
		currentTokens += paraTokens

		// Keep last N lines for overlap
		allLines := strings.Split(strings.Join(currentLines, "\n"), "\n")
		if len(allLines) > overlapLines {
			overlapBuffer = allLines[len(allLines)-overlapLines:]
		} else {
			overlapBuffer = allLines
		}
	}

	// Emit final chunk
	if len(currentLines) > 0 {
		chunks = append(chunks, Chunk{
			FileName: fi.Path,
			Content:  preamble + strings.Join(currentLines, "\n"),
			FileInfo: fi,
			Pass:     2,
		})
	}

	// Set Index/Total on all chunks
	for i := range chunks {
		chunks[i].Index = i
		chunks[i].Total = len(chunks)
	}

	return chunks, nil
}

var divisionRegex = regexp.MustCompile(`(?im)^\s{6}\s+(IDENTIFICATION|ENVIRONMENT|DATA|PROCEDURE)\s+DIVISION`)

// splitDivisions splits COBOL content into its four divisions.
func splitDivisions(content string) map[string]string {
	divs := make(map[string]string)
	locs := divisionRegex.FindAllStringSubmatchIndex(content, -1)

	if len(locs) == 0 {
		divs["PROCEDURE"] = content
		return divs
	}

	for i, loc := range locs {
		name := strings.ToUpper(content[loc[2]:loc[3]])
		start := loc[0]
		var end int
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else {
			end = len(content)
		}
		divs[name] = content[start:end]
	}

	return divs
}

type paragraphUnit struct {
	name    string
	content string
}

var paragraphRegex = regexp.MustCompile(`(?m)^.{6} ([A-Za-z0-9][A-Za-z0-9-]+)\.\s*$`)
var sectionRegex = regexp.MustCompile(`(?im)^.{6} ([A-Z][A-Z0-9-]+)\s+SECTION\.\s*$`)

// splitParagraphs splits the PROCEDURE DIVISION into paragraph/section units.
func splitParagraphs(procedure string) []paragraphUnit {
	lines := strings.Split(procedure, "\n")

	type boundary struct {
		lineIdx int
		name    string
	}

	var boundaries []boundary

	for i, line := range lines {
		if sectionRegex.MatchString(line) {
			subs := sectionRegex.FindStringSubmatch(line)
			if len(subs) >= 2 {
				boundaries = append(boundaries, boundary{lineIdx: i, name: subs[1] + " SECTION"})
				continue
			}
		}
		if paragraphRegex.MatchString(line) {
			subs := paragraphRegex.FindStringSubmatch(line)
			if len(subs) >= 2 {
				boundaries = append(boundaries, boundary{lineIdx: i, name: subs[1]})
			}
		}
	}

	if len(boundaries) == 0 {
		return []paragraphUnit{{name: "PROCEDURE", content: procedure}}
	}

	var units []paragraphUnit
	for i, b := range boundaries {
		start := b.lineIdx
		var end int
		if i+1 < len(boundaries) {
			end = boundaries[i+1].lineIdx
		} else {
			end = len(lines)
		}
		units = append(units, paragraphUnit{
			name:    b.name,
			content: strings.Join(lines[start:end], "\n"),
		})
	}

	// Include any content before the first paragraph
	if boundaries[0].lineIdx > 0 {
		pre := strings.Join(lines[:boundaries[0].lineIdx], "\n")
		if strings.TrimSpace(pre) != "" {
			units = append([]paragraphUnit{{name: "PREAMBLE", content: pre}}, units...)
		}
	}

	return units
}
