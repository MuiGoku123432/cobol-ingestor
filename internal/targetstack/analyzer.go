package targetstack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"text/template"

	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	"cobol-ingestor/prompts"

	"go.uber.org/zap"
)

const maxFileBytes = 200_000 // skip files over ~200 KB

// Analyzer performs per-file LLM extraction and cross-file synthesis.
type Analyzer struct {
	provider   llm.ChatProvider
	model      string
	maxTokens  int
	tokenLimit int
	pass2Batch int
	pass2Toks  int
	extractTpl *template.Template
	synthTpl   *template.Template
	logger     *zap.Logger
}

// NewAnalyzer creates an Analyzer with the given LLM provider.
func NewAnalyzer(provider llm.ChatProvider, model string, maxTokens, tokenLimit, pass2Batch, pass2MaxTokens int, logger *zap.Logger) (*Analyzer, error) {
	extractTpl, err := template.New("ts_extract").Parse(prompts.TSExtract)
	if err != nil {
		return nil, fmt.Errorf("parsing ts_extract template: %w", err)
	}
	synthTpl, err := template.New("ts_synthesis").Parse(prompts.TSSynthesis)
	if err != nil {
		return nil, fmt.Errorf("parsing ts_synthesis template: %w", err)
	}
	return &Analyzer{
		provider:   provider,
		model:      model,
		maxTokens:  maxTokens,
		tokenLimit: tokenLimit,
		pass2Batch: pass2Batch,
		pass2Toks:  pass2MaxTokens,
		extractTpl: extractTpl,
		synthTpl:   synthTpl,
		logger:     logger,
	}, nil
}

// AnalyzeFiles performs Pass 1: per-file extraction with worker pool + SHA cache.
// Returns one ExtractionResult per file (only files whose hash changed since last run).
func (a *Analyzer) AnalyzeFiles(ctx context.Context, files []ScannedFile, repoURL string, fileCache *cache.Cache, maxWorkers int) ([]*ExtractionResult, error) {
	type workItem struct {
		f ScannedFile
	}
	type resultItem struct {
		result *ExtractionResult
		file   ScannedFile
		err    error
	}

	// Cache check
	pathHashes := make(map[string]string, len(files))
	for _, f := range files {
		pathHashes[f.Path] = f.Hash
	}
	changedPaths, err := fileCache.BatchIsChangedForPass(pathHashes, PassExtract)
	if err != nil {
		a.logger.Warn("batch cache check failed, processing all", zap.Error(err))
		changedPaths = make([]string, 0, len(files))
		for _, f := range files {
			changedPaths = append(changedPaths, f.Path)
		}
	}

	changedSet := make(map[string]bool, len(changedPaths))
	for _, p := range changedPaths {
		changedSet[p] = true
	}

	var workItems []workItem
	for _, f := range files {
		if f.Category == FileCategoryTest || f.Category == FileCategoryBuild {
			continue
		}
		if changedSet[f.Path] {
			workItems = append(workItems, workItem{f})
		}
	}

	a.logger.Info("target stack extraction",
		zap.Int("to_analyze", len(workItems)),
		zap.Int("cached_skip", len(files)-len(workItems)),
	)

	if len(workItems) == 0 {
		return nil, nil
	}

	sem := make(chan struct{}, maxWorkers)
	resultCh := make(chan resultItem, len(workItems))
	var wg sync.WaitGroup

	for _, wi := range workItems {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(w workItem) {
			defer wg.Done()
			defer func() { <-sem }()

			res, extractErr := a.analyzeFile(ctx, w.f, repoURL)
			resultCh <- resultItem{result: res, file: w.f, err: extractErr}
		}(wi)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []*ExtractionResult
	for item := range resultCh {
		if item.err != nil {
			a.logger.Error("file analysis failed", zap.String("path", item.file.Path), zap.Error(item.err))
			continue
		}
		if item.result != nil {
			results = append(results, item.result)
		}
		// Mark as processed even on empty results
		if cacheErr := fileCache.MarkProcessedForPass(item.file.Path, item.file.Hash, PassExtract); cacheErr != nil {
			a.logger.Warn("cache mark failed", zap.String("path", item.file.Path), zap.Error(cacheErr))
		}
	}

	return results, nil
}

// Synthesize performs Pass 2: cross-file synthesis of extracted entities.
// Batches extraction results and asks the LLM to merge and consolidate.
func (a *Analyzer) Synthesize(ctx context.Context, results []*ExtractionResult, repoName, language, framework string) (*graph.TargetStackResult, error) {
	if len(results) == 0 {
		return &graph.TargetStackResult{}, nil
	}

	// Flatten all extracted entities
	var allServices []graph.TargetService
	var allEndpoints []graph.TargetEndpoint
	var allRules []graph.TargetBusinessRule
	var allModels []graph.TargetDataModel
	var allIntegrations []graph.TargetIntegration
	var allErrorHandlers []graph.TargetErrorHandler

	for _, r := range results {
		allServices = append(allServices, r.Services...)
		allEndpoints = append(allEndpoints, r.Endpoints...)
		allRules = append(allRules, r.Rules...)
		allModels = append(allModels, r.DataModels...)
		allIntegrations = append(allIntegrations, r.Integrations...)
		allErrorHandlers = append(allErrorHandlers, r.ErrorHandlers...)
	}

	// Deduplicate by ID before synthesis
	allServices = deduplicateServices(allServices)
	allEndpoints = deduplicateEndpoints(allEndpoints)
	allRules = deduplicateRules(allRules)
	allModels = deduplicateModels(allModels)
	allIntegrations = deduplicateIntegrations(allIntegrations)
	allErrorHandlers = deduplicateErrorHandlers(allErrorHandlers)

	if len(allServices) == 0 {
		// Nothing extracted — return empty
		return &graph.TargetStackResult{}, nil
	}

	// Run synthesis in batches
	entitiesJSON, err := json.Marshal(map[string]any{
		"services":      allServices,
		"endpoints":     allEndpoints,
		"rules":         allRules,
		"dataModels":    allModels,
		"integrations":  allIntegrations,
		"errorHandlers": allErrorHandlers,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling entities: %w", err)
	}

	// Truncate if too large for the LLM context
	entityStr := string(entitiesJSON)
	if len(entityStr) > a.tokenLimit*3 {
		entityStr = entityStr[:a.tokenLimit*3] + "..."
	}

	var synthBuf bytes.Buffer
	if err := a.synthTpl.Execute(&synthBuf, map[string]any{
		"RepoName":    repoName,
		"Language":    language,
		"Framework":   framework,
		"EntityCount": len(allServices) + len(allEndpoints) + len(allRules) + len(allModels),
		"EntitiesJSON": entityStr,
	}); err != nil {
		return nil, fmt.Errorf("rendering synthesis template: %w", err)
	}

	resp, err := a.provider.CompleteChat(ctx, llm.ChatRequest{
		Model:     a.model,
		System:    "You are an enterprise software analyst performing a cross-file synthesis of extracted business logic. Return structured JSON.",
		MaxTokens: a.pass2Toks,
		Messages: []llm.ChatMessage{
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.NewTextContent(synthBuf.String())}},
		},
	})
	if err != nil {
		a.logger.Warn("synthesis LLM failed, returning pass 1 results", zap.Error(err))
		return &graph.TargetStackResult{
			Services:      allServices,
			Endpoints:     allEndpoints,
			Rules:         allRules,
			DataModels:    allModels,
			Integrations:  allIntegrations,
			ErrorHandlers: allErrorHandlers,
		}, nil
	}

	// Parse synthesis response (same schema as extraction)
	textResp := resp.TextContent()
	repoURL := ""
	if len(allServices) > 0 {
		repoURL = allServices[0].RepoURL
	}
	parsed, parseErr := ParseExtractionResponse(textResp, "(synthesis)", repoURL)
	if parseErr != nil {
		a.logger.Warn("synthesis parse failed, returning pass 1 results", zap.Error(parseErr))
		return &graph.TargetStackResult{
			Services:      allServices,
			Endpoints:     allEndpoints,
			Rules:         allRules,
			DataModels:    allModels,
			Integrations:  allIntegrations,
			ErrorHandlers: allErrorHandlers,
		}, nil
	}

	return &graph.TargetStackResult{
		Services:      parsed.Services,
		Endpoints:     parsed.Endpoints,
		Rules:         parsed.Rules,
		DataModels:    parsed.DataModels,
		Integrations:  parsed.Integrations,
		ErrorHandlers: parsed.ErrorHandlers,
	}, nil
}

// analyzeFile sends one source file to the LLM for business logic extraction.
func (a *Analyzer) analyzeFile(ctx context.Context, f ScannedFile, repoURL string) (*ExtractionResult, error) {
	if f.Size > maxFileBytes {
		a.logger.Debug("skipping oversized file", zap.String("path", f.Path), zap.Int64("bytes", f.Size))
		return &ExtractionResult{SourceFile: f.Path}, nil
	}

	content, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.Path, err)
	}

	// Truncate if token estimate too large (3 chars per token approximation)
	text := string(content)
	if estimatedTokens(text) > a.tokenLimit {
		cutoff := a.tokenLimit * tokenEstRatioInt
		if cutoff < len(text) {
			text = text[:cutoff]
		}
	}

	var promptBuf bytes.Buffer
	if err := a.extractTpl.Execute(&promptBuf, map[string]string{
		"FilePath": f.RelPath,
		"Language": f.Language,
		"Category": string(f.Category),
	}); err != nil {
		return nil, fmt.Errorf("rendering extract template for %s: %w", f.Path, err)
	}

	userMsg := promptBuf.String() + "\n\n---\n\n" + text

	resp, err := a.provider.CompleteChat(ctx, llm.ChatRequest{
		Model:     a.model,
		System:    "You are an enterprise software analyst extracting business logic from source code. Return structured JSON only.",
		MaxTokens: a.maxTokens,
		Messages: []llm.ChatMessage{
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.NewTextContent(userMsg)}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM analysis of %s: %w", f.Path, err)
	}

	result, parseErr := ParseExtractionResponse(resp.TextContent(), f.Path, repoURL)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing response for %s: %w", f.Path, parseErr)
	}

	return result, nil
}

const tokenEstRatioInt = 3 // conservative integer approximation for token/char ratio

func estimatedTokens(text string) int {
	return len(text) / tokenEstRatioInt
}

// --- dedup helpers ---

func deduplicateServices(items []graph.TargetService) []graph.TargetService {
	seen := make(map[string]bool)
	var out []graph.TargetService
	for _, s := range items {
		if !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	return out
}

func deduplicateEndpoints(items []graph.TargetEndpoint) []graph.TargetEndpoint {
	seen := make(map[string]bool)
	var out []graph.TargetEndpoint
	for _, s := range items {
		if !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	return out
}

func deduplicateRules(items []graph.TargetBusinessRule) []graph.TargetBusinessRule {
	seen := make(map[string]bool)
	var out []graph.TargetBusinessRule
	for _, s := range items {
		if !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	return out
}

func deduplicateModels(items []graph.TargetDataModel) []graph.TargetDataModel {
	seen := make(map[string]bool)
	var out []graph.TargetDataModel
	for _, s := range items {
		if !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	return out
}

func deduplicateIntegrations(items []graph.TargetIntegration) []graph.TargetIntegration {
	seen := make(map[string]bool)
	var out []graph.TargetIntegration
	for _, s := range items {
		if !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	return out
}

func deduplicateErrorHandlers(items []graph.TargetErrorHandler) []graph.TargetErrorHandler {
	seen := make(map[string]bool)
	var out []graph.TargetErrorHandler
	for _, s := range items {
		if !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	return out
}
