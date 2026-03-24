package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"

	"go.uber.org/zap"
)

// classifyBatchSize is the number of files sent per LLM classification call.
const classifyBatchSize = 12

// ClassifyPendingFiles resolves FileTypePending entries in result using LLM
// classification (when provider is non-nil) or heuristic fallback.
// Files classified as UNKNOWN are removed from result.Files.
func ClassifyPendingFiles(ctx context.Context, result *ScanResult, provider llm.Provider, model string, logger *zap.Logger) error {
	// Collect indices of pending files
	var pendingIdx []int
	for i, f := range result.Files {
		if f.Type == graph.FileTypePending {
			pendingIdx = append(pendingIdx, i)
		}
	}
	if len(pendingIdx) == 0 {
		return nil
	}

	logger.Info("classifying pending files",
		zap.Int("count", len(pendingIdx)),
		zap.Bool("llm", provider != nil),
	)

	// Build path->index map for updating in-place
	pathIdx := make(map[string]int, len(pendingIdx))
	for _, i := range pendingIdx {
		pathIdx[result.Files[i].Path] = i
	}

	var classifyErr error
	if provider != nil {
		classifyErr = classifyWithLLM(ctx, result, pathIdx, provider, model, logger)
	} else {
		classifyWithHeuristic(result, pathIdx, logger)
	}

	// Remove files still marked as PENDING (failed classification) and UNKNOWN
	filtered := result.Files[:0]
	for _, f := range result.Files {
		if f.Type == graph.FileTypePending {
			logger.Warn("file classification failed, skipping", zap.String("path", f.Path))
			delete(result.Snippets, f.Path)
			continue
		}
		if f.Type == "UNKNOWN" {
			logger.Debug("file is not mainframe source, skipping", zap.String("path", f.Path))
			delete(result.Snippets, f.Path)
			continue
		}
		filtered = append(filtered, f)
	}
	result.Files = filtered

	return classifyErr
}

// llmClassification is the JSON structure returned by the LLM.
type llmClassification struct {
	File string `json:"file"`
	Type string `json:"type"`
}

func classifyWithLLM(ctx context.Context, result *ScanResult, pathIdx map[string]int, provider llm.Provider, model string, logger *zap.Logger) error {
	// Collect pending paths in sorted order for deterministic batching
	paths := make([]string, 0, len(pathIdx))
	for p := range pathIdx {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var firstErr error
	for batchStart := 0; batchStart < len(paths); batchStart += classifyBatchSize {
		batchEnd := batchStart + classifyBatchSize
		if batchEnd > len(paths) {
			batchEnd = len(paths)
		}
		batch := paths[batchStart:batchEnd]

		// Build prompt
		var sb strings.Builder
		sb.WriteString("Classify each file snippet as one of: COBOL, COPYBOOK, JCL, or UNKNOWN.\n\n")
		sb.WriteString("Rules:\n")
		sb.WriteString("- COBOL: Complete COBOL programs with IDENTIFICATION/PROCEDURE DIVISION\n")
		sb.WriteString("- COPYBOOK: COBOL data definitions or code fragments meant to be INCLUDEd (no PROGRAM-ID)\n")
		sb.WriteString("- JCL: IBM Job Control Language (lines starting with //, JOB/EXEC/DD statements)\n")
		sb.WriteString("- UNKNOWN: Not mainframe source code\n\n")
		sb.WriteString("Respond with ONLY a JSON array: [{\"file\": \"filename\", \"type\": \"COBOL|COPYBOOK|JCL|UNKNOWN\"}]\n\n")

		for _, p := range batch {
			name := filepath.Base(p)
			sb.WriteString(fmt.Sprintf("=== File: %s ===\n", name))
			if snippet, ok := result.Snippets[p]; ok {
				for _, line := range snippet {
					sb.WriteString(line)
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}

		resp, err := provider.Complete(ctx, llm.CompletionRequest{
			Model: model,
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: sb.String()},
			},
			MaxTokens:   1024,
			Temperature: 0,
		})
		if err != nil {
			logger.Error("LLM classification batch failed, falling back to heuristic",
				zap.Int("batch_start", batchStart),
				zap.Error(err),
			)
			if firstErr == nil {
				firstErr = err
			}
			// Fall back to heuristic for this batch
			batchPathIdx := make(map[string]int, len(batch))
			for _, p := range batch {
				batchPathIdx[p] = pathIdx[p]
			}
			classifyWithHeuristic(result, batchPathIdx, logger)
			continue
		}

		// Parse JSON from response (strip markdown fences if present)
		body := strings.TrimSpace(resp.Content)
		if strings.HasPrefix(body, "```") {
			if idx := strings.Index(body[3:], "\n"); idx >= 0 {
				body = body[3+idx+1:]
			}
			if strings.HasSuffix(body, "```") {
				body = body[:len(body)-3]
			}
			body = strings.TrimSpace(body)
		}

		var classifications []llmClassification
		if err := json.Unmarshal([]byte(body), &classifications); err != nil {
			logger.Error("failed to parse LLM classification response, falling back to heuristic",
				zap.Error(err),
				zap.String("response", resp.Content),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("parsing LLM response: %w", err)
			}
			batchPathIdx := make(map[string]int, len(batch))
			for _, p := range batch {
				batchPathIdx[p] = pathIdx[p]
			}
			classifyWithHeuristic(result, batchPathIdx, logger)
			continue
		}

		// Map filename back to full path
		nameToPath := make(map[string]string, len(batch))
		for _, p := range batch {
			nameToPath[filepath.Base(p)] = p
		}

		for _, c := range classifications {
			p, ok := nameToPath[c.File]
			if !ok {
				continue
			}
			idx, ok := pathIdx[p]
			if !ok {
				continue
			}
			ft := mapClassificationType(c.Type)
			result.Files[idx].Type = ft
			delete(result.Snippets, p)
			logger.Info("LLM classified file",
				zap.String("file", c.File),
				zap.String("type", string(ft)),
			)
		}
	}

	return firstErr
}

func classifyWithHeuristic(result *ScanResult, pathIdx map[string]int, logger *zap.Logger) {
	for path, idx := range pathIdx {
		snippet := result.Snippets[path]
		if ft, ok := classifyByContent(snippet); ok {
			result.Files[idx].Type = ft
			logger.Info("heuristic classified file",
				zap.String("file", filepath.Base(path)),
				zap.String("type", string(ft)),
			)
		} else {
			result.Files[idx].Type = "UNKNOWN"
			logger.Debug("heuristic could not classify file",
				zap.String("file", filepath.Base(path)),
			)
		}
		delete(result.Snippets, path)
	}
}

// jclPattern matches JCL statements: //NAME JOB|EXEC|DD
var jclPattern = regexp.MustCompile(`^//\w+\s+(JOB|EXEC|DD)\s`)

// classifyByContent uses heuristic scoring to determine file type from content lines.
func classifyByContent(lines []string) (graph.FileType, bool) {
	if len(lines) == 0 {
		return "", false
	}

	var jclScore, cobolScore, copybookScore int
	hasProgramID := false

	for _, line := range lines {
		upper := strings.ToUpper(strings.TrimSpace(line))

		// JCL detection
		if jclPattern.MatchString(line) {
			jclScore++
		}

		// COBOL detection
		if strings.Contains(upper, "IDENTIFICATION DIVISION") {
			cobolScore++
		}
		if strings.Contains(upper, "PROGRAM-ID") {
			cobolScore++
			hasProgramID = true
		}
		if strings.Contains(upper, "PROCEDURE DIVISION") {
			cobolScore++
		}
		if strings.Contains(upper, "DATA DIVISION") {
			cobolScore++
		}
		if strings.Contains(upper, "ENVIRONMENT DIVISION") {
			cobolScore++
		}

		// Copybook detection: level numbers + PIC/REDEFINES
		if isLevelNumber(upper) {
			copybookScore++
		}
		if strings.Contains(upper, " PIC ") || strings.Contains(upper, " PICTURE ") {
			copybookScore++
		}
		if strings.Contains(upper, " REDEFINES ") {
			copybookScore++
		}
	}

	// JCL: 2+ JCL statement matches
	if jclScore >= 2 {
		return graph.FileTypeJCL, true
	}

	// COBOL: has PROGRAM-ID or division headers
	if cobolScore >= 1 && hasProgramID {
		return graph.FileTypeCOBOL, true
	}

	// Copybook: level numbers/PIC but no PROGRAM-ID — 3+ matches
	if copybookScore >= 3 && !hasProgramID {
		return graph.FileTypeCopybook, true
	}

	return "", false
}

// isLevelNumber checks if a trimmed uppercase line starts with a COBOL level number.
func isLevelNumber(upper string) bool {
	prefixes := []string{"01 ", "02 ", "03 ", "04 ", "05 ", "10 ", "15 ", "20 ", "25 ",
		"49 ", "66 ", "77 ", "88 "}
	for _, p := range prefixes {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

// mapClassificationType converts an LLM classification string to a graph.FileType.
func mapClassificationType(t string) graph.FileType {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "COBOL":
		return graph.FileTypeCOBOL
	case "COPYBOOK":
		return graph.FileTypeCopybook
	case "JCL":
		return graph.FileTypeJCL
	default:
		return "UNKNOWN"
	}
}
