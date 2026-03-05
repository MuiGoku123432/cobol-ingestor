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
// If the file exceeds the token limit, it is split at division boundaries,
// with each chunk receiving the IDENTIFICATION and ENVIRONMENT divisions as preamble.
// If a single division still exceeds the limit, PROCEDURE DIVISION is split at paragraph boundaries.
func ChunkFile(fi graph.FileInfo, tokenLimit int, logger *zap.Logger) ([]Chunk, error) {
	data, err := os.ReadFile(fi.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fi.Path, err)
	}

	content := string(data)
	tokens := EstimateTokens(content)

	// If it fits, return single chunk
	if tokens <= tokenLimit {
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

	logger.Info("file exceeds token limit, splitting into multiple chunks",
		zap.String("file", fi.Path),
		zap.Int("estimated_tokens", tokens),
		zap.Int("limit", tokenLimit),
	)

	// Split at division boundaries
	divs := splitDivisions(content)

	// Build preamble from IDENTIFICATION + ENVIRONMENT (always included in each chunk)
	preamble := ""
	for _, name := range []string{"IDENTIFICATION", "ENVIRONMENT"} {
		if div, ok := divs[name]; ok {
			preamble += div + "\n"
		}
	}

	var chunks []Chunk

	// Try to group DATA + PROCEDURE if they fit together
	dataPart := divs["DATA"]
	procPart := divs["PROCEDURE"]

	// If the whole file minus preamble fits, send as one (shouldn't happen since we checked above)
	remaining := ""
	if dataPart != "" {
		remaining += dataPart + "\n"
	}
	if procPart != "" {
		remaining += procPart
	}

	if EstimateTokens(preamble+remaining) <= tokenLimit {
		return []Chunk{
			{
				FileName: fi.Path,
				Content:  preamble + remaining,
				FileInfo: fi,
				Index:    0,
				Total:    1,
				Pass:     1,
			},
		}, nil
	}

	// DATA division as its own chunk if it exists and is non-trivial
	if dataPart != "" && EstimateTokens(dataPart) > 0 {
		chunks = append(chunks, Chunk{
			FileName: fi.Path,
			Content:  preamble + dataPart,
			FileInfo: fi,
			Pass:     1,
		})
	}

	// Handle PROCEDURE division — split at paragraph boundaries if needed
	if procPart != "" {
		preambleTokens := EstimateTokens(preamble)
		procTokens := EstimateTokens(procPart)
		budget := tokenLimit - preambleTokens

		if budget <= 0 {
			budget = tokenLimit / 2
		}

		if procTokens <= budget {
			chunks = append(chunks, Chunk{
				FileName: fi.Path,
				Content:  preamble + procPart,
				FileInfo: fi,
				Pass:     1,
			})
		} else {
			// Split PROCEDURE at paragraph boundaries
			paragraphs := splitParagraphs(procPart)
			var currentLines []string
			currentTokens := 0

			for _, para := range paragraphs {
				paraTokens := EstimateTokens(para.content)
				if currentTokens+paraTokens > budget && len(currentLines) > 0 {
					chunks = append(chunks, Chunk{
						FileName: fi.Path,
						Content:  preamble + strings.Join(currentLines, "\n"),
						FileInfo: fi,
						Pass:     1,
					})
					currentLines = nil
					currentTokens = 0
				}
				currentLines = append(currentLines, strings.Split(para.content, "\n")...)
				currentTokens += paraTokens
			}
			if len(currentLines) > 0 {
				chunks = append(chunks, Chunk{
					FileName: fi.Path,
					Content:  preamble + strings.Join(currentLines, "\n"),
					FileInfo: fi,
					Pass:     1,
				})
			}
		}
	}

	// If no chunks were created (edge case), return the full content as one chunk
	if len(chunks) == 0 {
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

	// Set Index/Total on all chunks
	for i := range chunks {
		chunks[i].Index = i
		chunks[i].Total = len(chunks)
	}

	return chunks, nil
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

// normalizeContinuations joins COBOL fixed-format continuation lines (column 7 = '-').
// A continuation line has '-' in column 7 (0-indexed column 6) and continues the previous line.
func normalizeContinuations(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		if len(line) >= 7 && line[6] == '-' {
			// This is a continuation line — append its content (from column 12 onward) to the previous line
			if len(result) > 0 {
				continuation := ""
				if len(line) > 11 {
					continuation = line[11:]
				} else if len(line) > 7 {
					continuation = strings.TrimLeft(line[7:], " ")
				}
				result[len(result)-1] = strings.TrimRight(result[len(result)-1], " ") + continuation
				continue
			}
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// copyRegex matches COPY statements including optional REPLACING clauses, up to the terminating period.
var copyRegex = regexp.MustCompile(`(?im)^\s+COPY\s+([A-Za-z0-9_-]+)\s*([^.]*?)\.`)

// replacingRegex parses REPLACING pairs: ==old== BY ==new==
var replacingRegex = regexp.MustCompile(`==\s*([^=]+?)\s*==\s+BY\s+==\s*([^=]+?)\s*==`)

// InlineCopybooks replaces COPY statements with copybook content.
// Tracks visited set to prevent circular references. maxDepth prevents runaway recursion.
// Normalizes continuation lines before regex matching to handle multi-line COPY statements.
func InlineCopybooks(content string, index CopybookIndex, maxDepth int) (string, error) {
	content = normalizeContinuations(content)
	return inlineCopybooksRecurse(content, index, maxDepth, make(map[string]bool))
}

// parseReplacingClause extracts replacement pairs from a REPLACING clause.
func parseReplacingClause(clause string) [][2]string {
	matches := replacingRegex.FindAllStringSubmatch(clause, -1)
	pairs := make([][2]string, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 3 {
			pairs = append(pairs, [2]string{m[1], m[2]})
		}
	}
	return pairs
}

// applyReplacements applies REPLACING substitutions to copybook content.
func applyReplacements(content string, pairs [][2]string) string {
	for _, pair := range pairs {
		content = strings.ReplaceAll(content, pair[0], pair[1])
	}
	return content
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

		copybookContent := string(data)

		// Apply REPLACING substitutions if present
		if len(subs) >= 3 && strings.Contains(strings.ToUpper(subs[2]), "REPLACING") {
			pairs := parseReplacingClause(subs[2])
			copybookContent = applyReplacements(copybookContent, pairs)
		}

		visited[name] = true
		inlined, _ := inlineCopybooksRecurse(copybookContent, index, depth-1, visited)
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

var divisionRegex = regexp.MustCompile(`(?im)^\s*(IDENTIFICATION|ENVIRONMENT|DATA|PROCEDURE)\s+DIVISION`)

// splitDivisions splits COBOL content into its four divisions.
// Normalizes continuation lines before matching division headers.
func splitDivisions(content string) map[string]string {
	content = normalizeContinuations(content)
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

var paragraphRegex = regexp.MustCompile(`(?im)^\s+([A-Za-z0-9][A-Za-z0-9_-]+)\.\s*$`)
var sectionRegex = regexp.MustCompile(`(?im)^\s+([A-Za-z][A-Za-z0-9_-]+)\s+SECTION\.\s*$`)

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
