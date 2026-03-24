package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"

	"go.uber.org/zap"
)

// classifyBatchSize is the number of files sent per LLM classification call.
const classifyBatchSize = 8

// classifyMaxWorkers is the max concurrent LLM classification requests.
const classifyMaxWorkers = 5

// ClassifyPendingFiles resolves FileTypePending entries in result using LLM
// classification (when provider is non-nil) or heuristic fallback.
// Files classified as UNKNOWN are removed from result.Files.
// classifyCache is optional (nil skips caching).
func ClassifyPendingFiles(ctx context.Context, result *ScanResult, provider llm.Provider, model string, logger *zap.Logger, classifyCache *cache.Cache) error {
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

	// Cache lookup: resolve files that haven't changed since last classification
	if classifyCache != nil {
		pathHashes := make(map[string]string, len(pathIdx))
		for path, idx := range pathIdx {
			pathHashes[path] = result.Files[idx].Hash
		}

		hits, _, err := classifyCache.BatchLookupClassification(pathHashes)
		if err != nil {
			logger.Warn("classify cache lookup failed, proceeding without cache", zap.Error(err))
		} else {
			for path, cr := range hits {
				idx := pathIdx[path]
				result.Files[idx].Type = graph.FileType(cr.FileType)
				delete(result.Snippets, path)
				delete(pathIdx, path)
				logger.Info("cached classification",
					zap.String("file", filepath.Base(path)),
					zap.String("type", cr.FileType),
					zap.String("classifier", cr.Classifier),
				)
			}
		}

		if len(pathIdx) == 0 {
			logger.Info("all pending files resolved from cache")
			// Still need to filter UNKNOWN files from cache hits
			return filterClassifiedFiles(result, logger)
		}
	}

	// Track classifier per file for cache storage
	classifiedBy := make(map[string]string, len(pathIdx))

	var classifyErr error
	if provider != nil {
		classifyErr = classifyWithLLM(ctx, result, pathIdx, provider, model, logger, classifiedBy)
	} else {
		classifyWithHeuristic(result, pathIdx, logger, classifiedBy)
	}

	// Store newly classified files in cache
	if classifyCache != nil {
		var entries []cache.ClassifyEntry
		for path, classifier := range classifiedBy {
			idx, ok := pathIdx[path]
			if !ok {
				continue
			}
			entries = append(entries, cache.ClassifyEntry{
				Path:       path,
				Hash:       result.Files[idx].Hash,
				FileType:   string(result.Files[idx].Type),
				Classifier: classifier,
			})
		}
		if len(entries) > 0 {
			if err := classifyCache.BatchMarkClassified(entries); err != nil {
				logger.Warn("failed to store classify cache", zap.Error(err))
			}
		}
	}

	return filterClassifiedFiles(result, logger, classifyErr)
}

// filterClassifiedFiles removes PENDING and UNKNOWN files from result.Files.
func filterClassifiedFiles(result *ScanResult, logger *zap.Logger, errs ...error) error {
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

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// llmClassification is the JSON structure returned by the LLM.
type llmClassification struct {
	File string `json:"file"`
	Type string `json:"type"`
}

func classifyWithLLM(ctx context.Context, result *ScanResult, pathIdx map[string]int, provider llm.Provider, model string, logger *zap.Logger, classifiedBy map[string]string) error {
	// Collect pending paths in sorted order for deterministic batching
	paths := make([]string, 0, len(pathIdx))
	for p := range pathIdx {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Build batches with pre-copied snippets (avoids concurrent map reads)
	type classifyBatch struct {
		paths    []string
		snippets map[string][]string
	}
	var batches []classifyBatch
	for i := 0; i < len(paths); i += classifyBatchSize {
		end := i + classifyBatchSize
		if end > len(paths) {
			end = len(paths)
		}
		batchPaths := paths[i:end]
		batchSnippets := make(map[string][]string, len(batchPaths))
		for _, p := range batchPaths {
			batchSnippets[p] = result.Snippets[p]
		}
		batches = append(batches, classifyBatch{paths: batchPaths, snippets: batchSnippets})
	}

	logger.Info("LLM classification starting",
		zap.Int("files", len(paths)),
		zap.Int("batches", len(batches)),
		zap.Int("workers", classifyMaxWorkers),
	)

	// Mutex protects result.Files and result.Snippets during concurrent updates
	var mu sync.Mutex
	var firstErr error
	var errOnce sync.Once

	sem := make(chan struct{}, classifyMaxWorkers)
	var wg sync.WaitGroup

	for batchIdx, b := range batches {
		wg.Add(1)
		go func(batchIdx int, batch []string, batchSnippets map[string][]string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			// Build prompt using pre-copied snippets
			prompt := buildClassifyPrompt(batch, batchSnippets)

			resp, err := provider.Complete(ctx, llm.CompletionRequest{
				Model: model,
				Messages: []llm.Message{
					{Role: llm.RoleSystem, Content: "You are an expert IBM mainframe and COBOL analyst. Your task is to classify source code files from mainframe COBOL codebases. When uncertain, prefer COBOL or COPYBOOK over UNKNOWN — it is better to include a borderline file than to miss real mainframe source code."},
					{Role: llm.RoleUser, Content: prompt},
				},
				MaxTokens:   1024,
				Temperature: 0,
			})
			if err != nil {
				logger.Error("LLM classification batch failed, falling back to heuristic",
					zap.Int("batch", batchIdx),
					zap.Error(err),
				)
				errOnce.Do(func() { firstErr = err })
				batchPathIdx := make(map[string]int, len(batch))
				for _, p := range batch {
					batchPathIdx[p] = pathIdx[p]
				}
				mu.Lock()
				classifyWithHeuristic(result, batchPathIdx, logger, classifiedBy)
				mu.Unlock()
				return
			}

			// Parse JSON from response
			classifications, parseErr := parseClassifyResponse(resp.Content)
			if parseErr != nil {
				logger.Error("failed to parse LLM classification response, falling back to heuristic",
					zap.Int("batch", batchIdx),
					zap.Error(parseErr),
					zap.String("response", resp.Content),
				)
				errOnce.Do(func() { firstErr = fmt.Errorf("parsing LLM response: %w", parseErr) })
				batchPathIdx := make(map[string]int, len(batch))
				for _, p := range batch {
					batchPathIdx[p] = pathIdx[p]
				}
				mu.Lock()
				classifyWithHeuristic(result, batchPathIdx, logger, classifiedBy)
				mu.Unlock()
				return
			}

			// Map filename back to full path
			nameToPath := make(map[string]string, len(batch))
			for _, p := range batch {
				nameToPath[filepath.Base(p)] = p
			}

			mu.Lock()
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
				classifiedBy[p] = "LLM"
				logger.Info("LLM classified file",
					zap.String("file", c.File),
					zap.String("type", string(ft)),
				)
			}
			mu.Unlock()
		}(batchIdx, b.paths, b.snippets)
	}

	wg.Wait()
	return firstErr
}

// buildClassifyPrompt builds the classification prompt for a batch of files.
func buildClassifyPrompt(batch []string, snippets map[string][]string) string {
	var sb strings.Builder
	sb.WriteString("Classify each file snippet by its mainframe source type.\n\n")
	sb.WriteString("Common types: COBOL, COPYBOOK, JCL, BMS, DCLGEN, ASM, PLI, REXX, NATURAL, PROC, CLIST\n")
	sb.WriteString("Use UNKNOWN only for files that are not mainframe/programming source code.\n")
	sb.WriteString("Return any type that accurately describes the source — you are not limited to the list above.\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- COBOL: Any file containing COBOL statements, division headers (IDENTIFICATION, ENVIRONMENT, DATA, PROCEDURE), or COBOL verbs (PERFORM, MOVE, CALL, EVALUATE, COMPUTE, IF/ELSE/END-IF, EXEC SQL, EXEC CICS). Does NOT require all four divisions — partial programs and single-division files count as COBOL.\n")
	sb.WriteString("- COPYBOOK: COBOL data definitions (level numbers with PIC/PICTURE), 88-level conditions, SQL host variable declarations (EXEC SQL INCLUDE), paragraph-level code fragments meant to be INCLUDEd — no PROGRAM-ID.\n")
	sb.WriteString("- JCL: IBM Job Control Language (lines starting with //, JOB/EXEC/DD statements)\n")
	sb.WriteString("- BMS: Basic Mapping Support macro definitions (DFHMSD, DFHMDI, DFHMDF)\n")
	sb.WriteString("- DCLGEN: DB2 DCLGEN output (EXEC SQL DECLARE TABLE, host variable copybooks)\n")
	sb.WriteString("- UNKNOWN: Clearly not mainframe/programming source code (e.g., plain English docs, XML, HTML). When uncertain, prefer a specific type over UNKNOWN.\n\n")
	sb.WriteString("Tiebreaker: If a file shows even one strong mainframe indicator, classify with a specific type, not UNKNOWN.\n\n")
	sb.WriteString("Respond with ONLY a JSON array: [{\"file\": \"filename\", \"type\": \"<TYPE>\"}]\n\n")

	for _, p := range batch {
		name := filepath.Base(p)
		sb.WriteString(fmt.Sprintf("=== File: %s ===\n", name))
		if snippet, ok := snippets[p]; ok {
			trimmed := trimCommentHeader(snippet)
			for _, line := range trimmed {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// trimCommentHeader strips leading COBOL comment lines (starting with '*' in column 7
// or '*' after trimming) and blank lines, so the LLM sees actual code sooner.
func trimCommentHeader(lines []string) []string {
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		// COBOL comment: '*' in column 7 (0-indexed col 6) or line starts with '*' after trim
		if strings.HasPrefix(trimmed, "*") {
			i++
			continue
		}
		break
	}
	if i >= len(lines) {
		return lines // all comments/blanks — return original so LLM has something
	}
	return lines[i:]
}

// parseClassifyResponse extracts classifications from the LLM response body.
func parseClassifyResponse(content string) ([]llmClassification, error) {
	body := strings.TrimSpace(content)
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
		return nil, err
	}
	return classifications, nil
}

func classifyWithHeuristic(result *ScanResult, pathIdx map[string]int, logger *zap.Logger, classifiedBy map[string]string) {
	for path, idx := range pathIdx {
		snippet := result.Snippets[path]
		if ft, ok := classifyByContent(snippet); ok {
			result.Files[idx].Type = ft
			classifiedBy[path] = "HEURISTIC"
			logger.Info("heuristic classified file",
				zap.String("file", filepath.Base(path)),
				zap.String("type", string(ft)),
			)
		} else {
			result.Files[idx].Type = "UNKNOWN"
			classifiedBy[path] = "HEURISTIC"
			logger.Debug("heuristic could not classify file",
				zap.String("file", filepath.Base(path)),
			)
		}
		delete(result.Snippets, path)
	}
}

// jclPattern matches JCL statements: //NAME JOB|EXEC|DD
var jclPattern = regexp.MustCompile(`^//\w+\s+(JOB|EXEC|DD)\s`)

// levelNumberRe matches valid COBOL level numbers (01-49, 66, 77, 88).
var levelNumberRe = regexp.MustCompile(`^(0[1-9]|[1-4][0-9]|66|77|88)\s+`)

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

		// COBOL division headers
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

		// COBOL section headers
		if strings.Contains(upper, "WORKING-STORAGE SECTION") {
			cobolScore++
		}
		if strings.Contains(upper, "LINKAGE SECTION") {
			cobolScore++
		}
		if strings.Contains(upper, "FILE SECTION") {
			cobolScore++
		}

		// COBOL statements / verbs
		if strings.Contains(upper, "EXEC SQL") {
			cobolScore++
		}
		if strings.Contains(upper, "EXEC CICS") {
			cobolScore++
		}
		if strings.Contains(upper, "PERFORM ") {
			cobolScore++
		}
		if strings.Contains(upper, "MOVE ") {
			cobolScore++
		}
		if strings.Contains(upper, "CALL '") {
			cobolScore++
		}
		if strings.Contains(upper, "EVALUATE ") {
			cobolScore++
		}
		if strings.Contains(upper, "GOBACK") {
			cobolScore++
		}
		if strings.Contains(upper, "STOP RUN") {
			cobolScore++
		}
		if strings.Contains(upper, "COPY ") {
			cobolScore++
		}

		// Copybook detection: level numbers + PIC/REDEFINES + data-definition keywords
		if isLevelNumber(upper) {
			copybookScore++
		}
		if strings.Contains(upper, " PIC ") || strings.Contains(upper, " PICTURE ") {
			copybookScore++
		}
		if strings.Contains(upper, " REDEFINES ") {
			copybookScore++
		}
		if strings.Contains(upper, " VALUE ") {
			copybookScore++
		}
		if strings.Contains(upper, " OCCURS ") {
			copybookScore++
		}
		if strings.Contains(upper, "COMP-3") || strings.Contains(upper, " COMP ") {
			copybookScore++
		}
		if strings.Contains(upper, " FILLER ") {
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

	// COBOL: 3+ strong signals even without PROGRAM-ID (partial programs)
	if cobolScore >= 3 && !hasProgramID {
		return graph.FileTypeCOBOL, true
	}

	// Copybook: level numbers/PIC but no PROGRAM-ID — 2+ matches
	if copybookScore >= 2 && !hasProgramID {
		return graph.FileTypeCopybook, true
	}

	// Weak fallback: any COBOL or copybook signal -> COPYBOOK (safer to over-include)
	if (cobolScore >= 1 || copybookScore >= 1) && !hasProgramID {
		return graph.FileTypeCopybook, true
	}

	return "", false
}

// isLevelNumber checks if a trimmed uppercase line starts with a valid COBOL level number.
func isLevelNumber(upper string) bool {
	return levelNumberRe.MatchString(upper)
}

// mapClassificationType converts an LLM classification string to a graph.FileType.
// Passes through any non-empty type as-is (uppercased), only defaulting to UNKNOWN if empty.
func mapClassificationType(t string) graph.FileType {
	upper := strings.ToUpper(strings.TrimSpace(t))
	if upper == "" {
		return "UNKNOWN"
	}
	return graph.FileType(upper)
}
