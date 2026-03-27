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

// classifyBatchSize is the default number of files sent per LLM classification call.
const classifyBatchSize = 8

// classifyMaxWorkers is the max concurrent LLM classification requests.
const classifyMaxWorkers = 5

// confidenceThreshold is the minimum confidence for accepting a classification.
const confidenceThreshold = 0.7

// retrySnippetLines defines snippet sizes for each attempt.
var retrySnippetLines = [3]int{150, 400, 800}

// retryBatchSizes defines batch sizes for each attempt.
var retryBatchSizes = [3]int{8, 4, 1}

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
				result.Files[idx].Confidence = cr.Confidence
				result.Files[idx].Classifier = "CACHE"
				delete(result.Snippets, path)
				delete(pathIdx, path)
				logger.Info("cached classification",
					zap.String("file", filepath.Base(path)),
					zap.String("type", cr.FileType),
					zap.String("classifier", cr.Classifier),
					zap.Float64("confidence", cr.Confidence),
				)
			}
		}

		if len(pathIdx) == 0 {
			logger.Info("all pending files resolved from cache")
			// Still need to filter UNKNOWN files from cache hits
			return filterClassifiedFiles(result, logger)
		}
	}

	// Track classifier + confidence per file for cache storage
	classifiedBy := make(map[string]string, len(pathIdx))
	classifiedConf := make(map[string]float64, len(pathIdx))

	var classifyErr error
	if provider != nil {
		classifyErr = classifyWithLLMProgressive(ctx, result, pathIdx, provider, model, logger, classifiedBy, classifiedConf)
	} else {
		classifyWithHeuristic(result, pathIdx, logger, classifiedBy, classifiedConf)
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
				Confidence: classifiedConf[path],
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
	File       string  `json:"file"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// classifyWithLLMProgressive implements 3-attempt progressive classification.
// Attempt 1: standard snippet, batch size 8
// Attempt 2: 400-line snippet, batch size 4 (files with confidence < 0.7 or UNKNOWN)
// Attempt 3: 800-line snippet, batch size 1 (still uncertain)
// Heuristic fallback only for files where all LLM attempts errored.
func classifyWithLLMProgressive(ctx context.Context, result *ScanResult, pathIdx map[string]int, provider llm.Provider, model string, logger *zap.Logger, classifiedBy map[string]string, classifiedConf map[string]float64) error {
	// Collect pending paths in sorted order for deterministic batching
	allPaths := make([]string, 0, len(pathIdx))
	for p := range pathIdx {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)

	// Track previous attempt results for enhanced retry prompts
	prevResults := make(map[string]*llmClassification)

	// Track files that errored on all attempts (candidates for heuristic)
	llmErrorFiles := make(map[string]bool)

	var firstErr error
	var errOnce sync.Once

	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			break
		}

		var pendingPaths []string
		if attempt == 0 {
			pendingPaths = allPaths
		} else {
			// Collect files that need retry: low confidence, UNKNOWN, or errored
			for _, p := range allPaths {
				idx := pathIdx[p]
				ft := result.Files[idx].Type
				conf := result.Files[idx].Confidence

				// Still pending (errored) or low confidence or UNKNOWN
				if ft == graph.FileTypePending || ft == "UNKNOWN" || conf < confidenceThreshold {
					pendingPaths = append(pendingPaths, p)
				}
			}
		}

		if len(pendingPaths) == 0 {
			break
		}

		batchSize := retryBatchSizes[attempt]
		snippetLines := retrySnippetLines[attempt]

		logger.Info("LLM classification attempt",
			zap.Int("attempt", attempt+1),
			zap.Int("files", len(pendingPaths)),
			zap.Int("snippetLines", snippetLines),
			zap.Int("batchSize", batchSize),
		)

		// For attempts 2+, re-read files with larger snippets
		if attempt > 0 {
			for _, p := range pendingPaths {
				newSnippet, err := readLargerSnippet(p, snippetLines)
				if err != nil {
					logger.Warn("failed to re-read file for retry",
						zap.String("file", filepath.Base(p)),
						zap.Error(err),
					)
					continue
				}
				if result.Snippets == nil {
					result.Snippets = make(map[string][]string)
				}
				result.Snippets[p] = newSnippet
			}
		}

		// Build batches
		type classifyBatch struct {
			paths    []string
			snippets map[string][]string
		}
		var batches []classifyBatch
		for i := 0; i < len(pendingPaths); i += batchSize {
			end := i + batchSize
			if end > len(pendingPaths) {
				end = len(pendingPaths)
			}
			batchPaths := pendingPaths[i:end]
			batchSnippets := make(map[string][]string, len(batchPaths))
			for _, p := range batchPaths {
				batchSnippets[p] = result.Snippets[p]
			}
			batches = append(batches, classifyBatch{paths: batchPaths, snippets: batchSnippets})
		}

		// Process batches concurrently
		var mu sync.Mutex
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

				// Build prompt (enhanced for retry attempts)
				mu.Lock()
				var batchPrevResults map[string]*llmClassification
				if attempt > 0 {
					batchPrevResults = make(map[string]*llmClassification)
					for _, p := range batch {
						if prev, ok := prevResults[p]; ok {
							batchPrevResults[p] = prev
						}
					}
				}
				mu.Unlock()

				prompt := buildClassifyPrompt(batch, batchSnippets)
				systemMsg := classifySystemMessage

				if attempt > 0 && len(batchPrevResults) > 0 {
					prompt = buildRetryClassifyPrompt(batch, batchSnippets, batchPrevResults, attempt+1)
				}

				resp, err := provider.Complete(ctx, llm.CompletionRequest{
					Model: model,
					Messages: []llm.Message{
						{Role: llm.RoleSystem, Content: systemMsg},
						{Role: llm.RoleUser, Content: prompt},
					},
					MaxTokens:   2048,
					Temperature: 0,
				})
				if err != nil {
					logger.Error("LLM classification batch failed",
						zap.Int("attempt", attempt+1),
						zap.Int("batch", batchIdx),
						zap.Error(err),
					)
					errOnce.Do(func() { firstErr = err })
					mu.Lock()
					for _, p := range batch {
						llmErrorFiles[p] = true
					}
					mu.Unlock()
					return
				}

				// Parse JSON from response
				classifications, parseErr := parseClassifyResponse(resp.Content)
				if parseErr != nil {
					logger.Error("failed to parse LLM classification response",
						zap.Int("attempt", attempt+1),
						zap.Int("batch", batchIdx),
						zap.Error(parseErr),
						zap.String("response", resp.Content),
					)
					errOnce.Do(func() { firstErr = fmt.Errorf("parsing LLM response: %w", parseErr) })
					mu.Lock()
					for _, p := range batch {
						llmErrorFiles[p] = true
					}
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
					conf := c.Confidence
					if conf == 0 {
						conf = 0.8 // default if LLM didn't provide confidence
					}

					// Only update if this attempt's result is better
					if attempt == 0 || conf > result.Files[idx].Confidence || result.Files[idx].Type == graph.FileTypePending {
						result.Files[idx].Type = ft
						result.Files[idx].Confidence = conf
						result.Files[idx].Classifier = "LLM"
						classifiedBy[p] = "LLM"
						classifiedConf[p] = conf
						delete(llmErrorFiles, p) // clear error flag on success
					}

					// Store for potential retry prompt
					prevResults[p] = &c

					logger.Info("LLM classified file",
						zap.Int("attempt", attempt+1),
						zap.String("file", c.File),
						zap.String("type", string(ft)),
						zap.Float64("confidence", conf),
						zap.String("reasoning", c.Reasoning),
					)
				}
				mu.Unlock()
			}(batchIdx, b.paths, b.snippets)
		}

		wg.Wait()
	}

	// Heuristic fallback ONLY for files that errored on ALL LLM attempts
	var heuristicPaths []string
	for _, p := range allPaths {
		idx := pathIdx[p]
		if result.Files[idx].Type == graph.FileTypePending && llmErrorFiles[p] {
			heuristicPaths = append(heuristicPaths, p)
		}
	}
	if len(heuristicPaths) > 0 {
		logger.Info("falling back to heuristic for LLM-errored files",
			zap.Int("count", len(heuristicPaths)),
		)
		heuristicIdx := make(map[string]int, len(heuristicPaths))
		for _, p := range heuristicPaths {
			heuristicIdx[p] = pathIdx[p]
		}
		classifyWithHeuristic(result, heuristicIdx, logger, classifiedBy, classifiedConf)
	}

	// Clean up snippets for all classified files
	for _, p := range allPaths {
		delete(result.Snippets, p)
	}

	return firstErr
}

// classifySystemMessage is the system prompt for LLM classification.
const classifySystemMessage = `You are an expert IBM mainframe and COBOL analyst with decades of experience classifying source code from mainframe COBOL codebases that have been migrated to flat files. Files have lost their original extensions and are wrapped in .txt or other generic extensions. You must determine the original source type from the content.

When uncertain, prefer COBOL or COPYBOOK over UNKNOWN — it is better to include a borderline file than to miss real mainframe source code.
Never classify a file as UNKNOWN if it contains any structured code, data definitions, or mainframe-related content.`

// buildClassifyPrompt builds the classification prompt for a batch of files.
func buildClassifyPrompt(batch []string, snippets map[string][]string) string {
	var sb strings.Builder
	sb.WriteString("Classify each file snippet by its mainframe source type.\n\n")

	sb.WriteString("## Valid Types\n")
	sb.WriteString("COBOL, COPYBOOK, JCL, BMS, DCLGEN, ASM, PLI, REXX, NATURAL, PROC, CLIST\n")
	sb.WriteString("Use UNKNOWN only for files that are clearly not mainframe/programming source code.\n")
	sb.WriteString("Return any type that accurately describes the source — you are not limited to the list above.\n")
	sb.WriteString("Examples of other valid types: CONTROL, DATA, EASYTRIEVE, IDMS, ADABAS, SORT, UTILITY, SCRIPT, MACRO.\n\n")

	sb.WriteString("## Classification Rules & Signals\n\n")

	sb.WriteString("**COBOL** (strong signals: PROGRAM-ID, division headers, PROCEDURE DIVISION + verbs):\n")
	sb.WriteString("- Any file containing COBOL statements, division headers (IDENTIFICATION, ENVIRONMENT, DATA, PROCEDURE)\n")
	sb.WriteString("- COBOL verbs: PERFORM, MOVE, CALL, EVALUATE, COMPUTE, IF/ELSE/END-IF, EXEC SQL, EXEC CICS, GOBACK, STOP RUN\n")
	sb.WriteString("- Does NOT require all four divisions — partial programs and single-division files count as COBOL\n")
	sb.WriteString("- Strong: PROGRAM-ID present → 0.95+ confidence\n")
	sb.WriteString("- Medium: 2+ division headers or PROCEDURE DIVISION + verbs → 0.8+ confidence\n\n")

	sb.WriteString("**COPYBOOK** (strong signals: level numbers with PIC, no PROGRAM-ID):\n")
	sb.WriteString("- COBOL data definitions: level numbers (01-49, 66, 77, 88) with PIC/PICTURE clauses\n")
	sb.WriteString("- 88-level conditions, REDEFINES, OCCURS, VALUE clauses\n")
	sb.WriteString("- SQL host variable declarations (EXEC SQL INCLUDE)\n")
	sb.WriteString("- Paragraph-level code fragments meant to be INCLUDEd — no PROGRAM-ID\n")
	sb.WriteString("- Strong: Multiple level numbers + PIC clauses, no PROGRAM-ID → 0.9+ confidence\n\n")

	sb.WriteString("**JCL** (strong signals: lines starting with //, JOB/EXEC/DD):\n")
	sb.WriteString("- IBM Job Control Language (lines starting with //)\n")
	sb.WriteString("- JOB, EXEC, DD statements; PROC/PEND; SET symbols\n")
	sb.WriteString("- Strong: 2+ JCL statements → 0.9+ confidence\n\n")

	sb.WriteString("**BMS** (strong signals: DFHMSD, DFHMDI, DFHMDF macros):\n")
	sb.WriteString("- Basic Mapping Support macro definitions\n\n")

	sb.WriteString("**DCLGEN** (strong signals: EXEC SQL DECLARE TABLE, generated host variables):\n")
	sb.WriteString("- DB2 DCLGEN output with EXEC SQL DECLARE TABLE statements\n\n")

	sb.WriteString("**UNKNOWN**: Use ONLY for files that contain NO structured content, code, or data definitions whatsoever — pure English prose documentation, XML/HTML markup, or CSV data with no mainframe indicators. NEVER return UNKNOWN if the file contains ANY code-like structure, level numbers, verbs, or column-based formatting.\n\n")

	sb.WriteString("## Confidence Rubric\n")
	sb.WriteString("- 0.95-1.0: Multiple strong signals unambiguously identify the type\n")
	sb.WriteString("- 0.80-0.94: Clear signals present, high certainty\n")
	sb.WriteString("- 0.70-0.79: Some signals present but could be ambiguous\n")
	sb.WriteString("- 0.50-0.69: Weak signals, uncertain classification\n")
	sb.WriteString("- Below 0.50: Weak signals — provide your best-guess type (do NOT default to UNKNOWN)\n\n")

	sb.WriteString("## Short File Guidance\n")
	sb.WriteString("For very short files (under 20 lines), classify based on whatever signals exist — even a single line with a COBOL verb, level number, JCL statement, or column-based formatting is sufficient to assign a type. Short does not mean UNKNOWN.\n\n")

	sb.WriteString("## Tiebreaker Rules\n")
	sb.WriteString("If a file shows even one strong mainframe indicator, classify with a specific type, not UNKNOWN.\n")
	sb.WriteString("Priority: COBOL > COPYBOOK > JCL > UNKNOWN\n\n")

	sb.WriteString("## Response Format\n")
	sb.WriteString("Respond with ONLY a JSON array:\n")
	sb.WriteString(`[{"file": "NAME.txt", "type": "COBOL", "confidence": 0.95, "reasoning": "PROGRAM-ID present, PROCEDURE DIVISION with PERFORM/CALL verbs"}]`)
	sb.WriteString("\n\n")

	for _, p := range batch {
		name := filepath.Base(p)
		hint := filenameHint(name)
		if hint != "" {
			sb.WriteString(fmt.Sprintf("=== File: %s (%s) ===\n", name, hint))
		} else {
			sb.WriteString(fmt.Sprintf("=== File: %s ===\n", name))
		}
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

// buildRetryClassifyPrompt builds an enhanced prompt for retry attempts,
// including previous attempt results and explicit uncertainty guidance.
func buildRetryClassifyPrompt(batch []string, snippets map[string][]string, prevResults map[string]*llmClassification, attemptNum int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Classification Retry (Attempt %d)\n\n", attemptNum))
	sb.WriteString("These files were uncertain in a previous classification attempt. More context is provided below.\n")
	sb.WriteString("Look harder for subtle signals — column-based COBOL formatting, section names, verb patterns, data names with hyphens.\n")
	sb.WriteString("Do NOT return UNKNOWN. Provide your best-guess classification even if confidence is low. If you previously returned UNKNOWN, look again for ANY structural or code-like signals and assign a specific type.\n\n")

	sb.WriteString("## Previous Results\n")
	for _, p := range batch {
		name := filepath.Base(p)
		if prev, ok := prevResults[p]; ok {
			sb.WriteString(fmt.Sprintf("- %s: previously classified as %s (confidence: %.2f)\n", name, prev.Type, prev.Confidence))
			if prev.Reasoning != "" {
				sb.WriteString(fmt.Sprintf("  Reasoning: %s\n", prev.Reasoning))
			}
		}
	}
	sb.WriteString("\n")

	// Include the same classification rules and confidence rubric
	sb.WriteString("## Valid Types\n")
	sb.WriteString("COBOL, COPYBOOK, JCL, BMS, DCLGEN, ASM, PLI, REXX, NATURAL, PROC, CLIST\n")
	sb.WriteString("Use UNKNOWN only for files that are clearly not mainframe/programming source code.\n\n")

	sb.WriteString("## Confidence Rubric\n")
	sb.WriteString("- 0.95-1.0: Multiple strong signals unambiguously identify the type\n")
	sb.WriteString("- 0.80-0.94: Clear signals present, high certainty\n")
	sb.WriteString("- 0.70-0.79: Some signals present but could be ambiguous\n")
	sb.WriteString("- 0.50-0.69: Weak signals, uncertain classification\n")
	sb.WriteString("- Below 0.50: Weak signals — provide your best-guess type (do NOT default to UNKNOWN)\n\n")

	sb.WriteString("## Tiebreaker: COBOL > COPYBOOK > JCL > UNKNOWN\n\n")

	sb.WriteString("## Response Format\n")
	sb.WriteString(`Respond with ONLY a JSON array: [{"file": "NAME.txt", "type": "COBOL", "confidence": 0.95, "reasoning": "..."}]`)
	sb.WriteString("\n\n")

	for _, p := range batch {
		name := filepath.Base(p)
		hint := filenameHint(name)
		if hint != "" {
			sb.WriteString(fmt.Sprintf("=== File: %s (%s) ===\n", name, hint))
		} else {
			sb.WriteString(fmt.Sprintf("=== File: %s ===\n", name))
		}
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

// filenameHint returns a classification hint based on the filename pattern.
func filenameHint(name string) string {
	upper := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))

	switch {
	case strings.HasPrefix(upper, "CPY") || strings.HasPrefix(upper, "COPY"):
		return "name suggests: COPYBOOK"
	case strings.HasPrefix(upper, "DCL"):
		return "name suggests: DCLGEN"
	case strings.HasPrefix(upper, "MAP") || strings.HasPrefix(upper, "BMS"):
		return "name suggests: BMS"
	case strings.HasPrefix(upper, "PROC"):
		return "name suggests: PROC"
	}

	// 8-char uppercase PDS member name pattern
	if len(upper) <= 8 && pdsNameRe.MatchString(upper) {
		return "PDS member name format"
	}

	return ""
}

// pdsNameRe matches valid PDS member names (1-8 chars, uppercase alphanumeric + @#$).
var pdsNameRe = regexp.MustCompile(`^[A-Z@#$][A-Z0-9@#$]{0,7}$`)

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

// extractJSONBlock scans s for the first balanced JSON array or object,
// respecting string literals and escape characters. It returns the balanced
// substring and true, or ("", false) if no balanced block is found.
func extractJSONBlock(s string) (string, bool) {
	var inString bool
	var escaped bool
	var stack []byte
	start := -1

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
		case '[', '{':
			if start == -1 {
				start = i
			}
			stack = append(stack, c)
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
				if len(stack) == 0 {
					return s[start : i+1], true
				}
			}
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
				if len(stack) == 0 {
					return s[start : i+1], true
				}
			}
		}
	}
	return "", false
}

// parseClassifyResponse extracts classifications from the LLM response body.
func parseClassifyResponse(content string) ([]llmClassification, error) {
	body := strings.TrimSpace(content)

	// Strip markdown code fences
	if strings.HasPrefix(body, "```") {
		if idx := strings.Index(body[3:], "\n"); idx >= 0 {
			body = body[3+idx+1:]
		}
		if strings.HasSuffix(body, "```") {
			body = body[:len(body)-3]
		}
		body = strings.TrimSpace(body)
	}

	// Try extracting a balanced JSON block (handles preamble AND trailing text).
	// Retry up to 10 times to skip false positives like [word] in prose.
	remaining := body
	for attempt := 0; attempt < 10; attempt++ {
		candidate, ok := extractJSONBlock(remaining)
		if !ok {
			break
		}

		// Handle single-object response: wrap in array
		toUnmarshal := candidate
		if len(candidate) > 0 && candidate[0] == '{' {
			toUnmarshal = "[" + candidate + "]"
		}

		var classifications []llmClassification
		if err := json.Unmarshal([]byte(toUnmarshal), &classifications); err == nil {
			return classifications, nil
		}

		// Advance past this candidate and try again
		idx := strings.Index(remaining, candidate)
		remaining = remaining[idx+len(candidate):]
	}

	return nil, fmt.Errorf("no valid JSON found in response")
}

func classifyWithHeuristic(result *ScanResult, pathIdx map[string]int, logger *zap.Logger, classifiedBy map[string]string, classifiedConf map[string]float64) {
	for path, idx := range pathIdx {
		snippet := result.Snippets[path]
		if ft, ok := classifyByContent(snippet); ok {
			result.Files[idx].Type = ft
			result.Files[idx].Confidence = 0.5
			result.Files[idx].Classifier = "HEURISTIC"
			classifiedBy[path] = "HEURISTIC"
			classifiedConf[path] = 0.5
			logger.Info("heuristic classified file",
				zap.String("file", filepath.Base(path)),
				zap.String("type", string(ft)),
			)
		} else {
			result.Files[idx].Type = "UNKNOWN"
			result.Files[idx].Classifier = "HEURISTIC"
			classifiedBy[path] = "HEURISTIC"
			classifiedConf[path] = 0.0
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
