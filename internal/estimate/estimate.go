// Package estimate computes pre-execution LLM cost estimates for the ingest pipeline.
// It scans and chunks files to produce exact estimates for Passes 1-2, and applies
// heuristic multipliers for Passes 3-5 where request counts depend on earlier pass results.
// No LLM calls are made.
package estimate

import (
	"math"
	"os"

	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/scanner"

	"go.uber.org/zap"
)

// Heuristic multipliers — tunable based on observed pipeline statistics.
const (
	// pass4CallPairRate covers LINKAGE, COMMAREA, and shared file flow pairs combined.
	pass4CallPairRate = 0.25 // fraction of COBOL programs with inter-program data flow calls

	// pass5RepairRate reflects that 30-50% of programs in large enterprise codebases
	// have gaps in CALLS, CHILD_OF, or MOVES_TO relationships after Passes 1-2.
	pass5RepairRate = 0.35

	// pass5AnnotationRepairRate reflects programs missing paragraph annotations.
	pass5AnnotationRepairRate = 0.20

	// pass5DeadVerifyRate: dead code detection flags ~15% of programs on average.
	pass5DeadVerifyRate = 0.15

	// pass5DomainMergeCalls: domain merge runs iteratively; 5 calls is a realistic estimate.
	pass5DomainMergeCalls = 5

	// outputTypicalFraction: Claude uses 70-80%+ of its output budget on dense COBOL analysis.
	outputTypicalFraction = 0.75

	// truncationRetryMultiplier accounts for completeWithRetry resending the full prompt.
	// Pass 2/Opus (32k max) truncates often (~2.5x). Pass 1/Sonnet rarely (~1.1x).
	// Blended conservative average across all passes.
	truncationRetryMultiplier = 1.8

	// Fixed prompt overhead in tokens per pass (system + template + examples).
	pass1PromptOverhead = 3018
	// pass2PromptOverhead includes template + example + Neo4j context preamble.
	// The preamble lists all paragraphs, data items, conditions, callers, callees from Pass 1.
	// For a program with ~80 paragraphs and ~200 data items it is ~3,000 tokens.
	pass2PromptOverhead    = 5878 // 2878 template/example + 3000 Neo4j context preamble
	pass3PromptOverhead    = 1441
	pass4PromptOverhead    = 354
	pass5RepairOverhead    = 2878 // repair uses pass2 template
	pass5VerifyOverhead    = 483  // pass5_dead_verify.tmpl overhead

	// pass3TokensPerProgram: each program entry in a graph slice includes callers, callees,
	// copybooks, paragraphs, file defs, SQL tables, CICS commands, external interfaces.
	pass3TokensPerProgram = 600

	// Pass 4 average input tokens per call pair (field context for both programs).
	pass4InputPerPair = 854

	// Scanner classification batch size.
	classifyBatchSize      = 8
	classifyMaxTokens      = 2048
	classifyPromptOverhead = 500 // per-batch prompt framing
	classifySnippetTokens  = 150 // tokens per file snippet
)

// PassEstimate holds the estimate for a single pipeline pass.
type PassEstimate struct {
	Name           string
	Model          string // "sonnet" or "opus"
	Requests       int
	InputTokens    int
	OutputTypical  int     // typical output tokens (50% of max * requests)
	OutputWorst    int     // worst-case output tokens (100% of max * requests)
	// Anthropic API pricing
	InputCost      float64
	OutputCostLow  float64
	OutputCostHigh float64
	// Copilot premium request pricing
	CopilotCost    float64
	Deterministic  bool   // false = heuristic estimate
	Notes          string
}

// ScannerEstimate holds the classification estimate for pending files.
type ScannerEstimate struct {
	PendingFiles   int
	Requests       int
	InputTokens    int
	OutputTypical  int
	OutputWorst    int
	InputCost      float64
	OutputCostLow  float64
	OutputCostHigh float64
	CopilotCost    float64
}

// FileCounts summarises file breakdown from the scan result.
type FileCounts struct {
	COBOL    int
	JCL      int
	Copybook int
	Pending  int
	Total    int
}

// CacheSkips reports how many files will be skipped per pass.
type CacheSkips struct {
	Pass1 int
	Pass2 int
}

// Totals aggregates across all passes.
type Totals struct {
	Requests       int
	InputTokens    int
	OutputTypical  int
	OutputWorst    int
	InputCost      float64
	OutputCostLow  float64
	OutputCostHigh float64
	CopilotCost    float64
}

// Result is the full pre-execution estimate.
type Result struct {
	SourceDir string
	Provider  string // "anthropic", "copilot", etc.
	Files     FileCounts
	Cache     CacheSkips
	Passes    []PassEstimate
	Scanner   ScannerEstimate
	Total     Totals
}

// Estimator runs the pre-execution cost estimation.
type Estimator struct {
	Config     *config.Config
	ScanResult *scanner.ScanResult
	Cache      *cache.Cache
	Logger     *zap.Logger
}

// New creates an Estimator.
func New(cfg *config.Config, sr *scanner.ScanResult, c *cache.Cache, logger *zap.Logger) *Estimator {
	return &Estimator{Config: cfg, ScanResult: sr, Cache: c, Logger: logger}
}

// Run computes the full estimate without making any LLM calls.
func (e *Estimator) Run() *Result {
	r := &Result{
		SourceDir: e.Config.Ingest.RootDir,
		Provider:  e.Config.LLM.Provider,
	}

	// Step A: classify files by type
	cobolFiles, jclFiles, copybookFiles, pendingFiles := e.classifyFiles()
	r.Files = FileCounts{
		COBOL:    len(cobolFiles),
		JCL:      len(jclFiles),
		Copybook: len(copybookFiles),
		Pending:  len(pendingFiles),
		Total:    len(e.ScanResult.Files),
	}

	// Step B: cache check — which files actually need processing
	changedCobol1 := e.cacheCheck(cobolFiles, 1)
	changedCobol2 := e.cacheCheck(cobolFiles, 2)
	r.Cache.Pass1 = len(cobolFiles) - len(changedCobol1)
	r.Cache.Pass2 = len(cobolFiles) - len(changedCobol2)

	// Build copybook index for Pass 2 chunking (reads copybook filenames only, no content)
	cbIndex := chunker.BuildCopybookIndex(e.ScanResult.Files)

	// Steps C-H: estimate each pass
	r.Passes = append(r.Passes, e.estimatePass1COBOL(changedCobol1))
	r.Passes = append(r.Passes, e.estimatePass1JCL(jclFiles))
	r.Passes = append(r.Passes, e.estimatePass2(changedCobol2, cbIndex))
	r.Passes = append(r.Passes, e.estimatePass3(len(cobolFiles)))
	r.Passes = append(r.Passes, e.estimatePass4(len(cobolFiles)))
	r.Passes = append(r.Passes, e.estimatePass5Repair(len(cobolFiles)))
	r.Passes = append(r.Passes, e.estimatePass5Annotations(len(cobolFiles)))
	r.Passes = append(r.Passes, e.estimatePass5Verify(len(cobolFiles)))

	// Step H: scanner classification for pending (content-detect) files
	r.Scanner = e.estimateScanner(len(pendingFiles))

	// Aggregate totals
	for _, p := range r.Passes {
		r.Total.Requests += p.Requests
		r.Total.InputTokens += p.InputTokens
		r.Total.OutputTypical += p.OutputTypical
		r.Total.OutputWorst += p.OutputWorst
		r.Total.InputCost += p.InputCost
		r.Total.OutputCostLow += p.OutputCostLow
		r.Total.OutputCostHigh += p.OutputCostHigh
		r.Total.CopilotCost += p.CopilotCost
	}
	if r.Scanner.Requests > 0 {
		r.Total.Requests += r.Scanner.Requests
		r.Total.InputTokens += r.Scanner.InputTokens
		r.Total.OutputTypical += r.Scanner.OutputTypical
		r.Total.OutputWorst += r.Scanner.OutputWorst
		r.Total.InputCost += r.Scanner.InputCost
		r.Total.OutputCostLow += r.Scanner.OutputCostLow
		r.Total.OutputCostHigh += r.Scanner.OutputCostHigh
		r.Total.CopilotCost += r.Scanner.CopilotCost
	}

	return r
}

func (e *Estimator) classifyFiles() (cobol, jcl, copybook, pending []graph.FileInfo) {
	for _, f := range e.ScanResult.Files {
		switch f.Type {
		case graph.FileTypeCOBOL:
			cobol = append(cobol, f)
		case graph.FileTypeJCL:
			jcl = append(jcl, f)
		case graph.FileTypeCopybook:
			copybook = append(copybook, f)
		case graph.FileTypePending:
			// Pending files (from --content-detect) haven't been LLM-classified yet.
			// For estimation purposes, treat them as COBOL — the common case when
			// content-detect is used. They will be processed through all 5 passes
			// after classification, same as regular .cbl files.
			pending = append(pending, f)
			cobol = append(cobol, f)
		}
	}
	return
}

// cacheCheck returns the subset of files that have changed.
// pass=1 checks file_cache; pass=2 checks pass_cache for pass 2.
func (e *Estimator) cacheCheck(files []graph.FileInfo, pass int) []graph.FileInfo {
	if e.Cache == nil || len(files) == 0 {
		return files
	}
	pathHashes := make(map[string]string, len(files))
	for _, f := range files {
		pathHashes[f.Path] = f.Hash
	}

	var changed []string
	var err error
	if pass == 1 {
		changed, err = e.Cache.BatchIsChanged(pathHashes)
	} else {
		changed, err = e.Cache.BatchIsChangedForPass(pathHashes, pass)
	}
	if err != nil {
		e.Logger.Warn("estimate: cache check failed, assuming all files changed", zap.Error(err))
		return files
	}

	changedSet := make(map[string]struct{}, len(changed))
	for _, p := range changed {
		changedSet[p] = struct{}{}
	}
	var result []graph.FileInfo
	for _, f := range files {
		if _, ok := changedSet[f.Path]; ok {
			result = append(result, f)
		}
	}
	return result
}

func (e *Estimator) estimatePass1COBOL(files []graph.FileInfo) PassEstimate {
	p := PassEstimate{
		Name:          "Pass 1 (Structural)",
		Model:         "sonnet",
		Deterministic: true,
	}
	maxOut := e.Config.Claude.Pass1MaxTokens

	for _, fi := range files {
		chunks, err := chunker.ChunkFile(fi, e.Config.Ingest.TokenLimit, e.Logger)
		if err != nil {
			e.Logger.Warn("estimate: chunk error", zap.String("file", fi.Path), zap.Error(err))
			// Fall back to size-based single chunk estimate
			p.Requests++
			tok := int(float64(fi.Size)/chunker.TokenEstimationRatio) + pass1PromptOverhead
			p.InputTokens += tok
			p.OutputTypical += int(float64(maxOut) * outputTypicalFraction)
			p.OutputWorst += maxOut
			continue
		}
		for _, c := range chunks {
			p.Requests++
			p.InputTokens += chunker.EstimateTokens(c.Content) + pass1PromptOverhead
			p.OutputTypical += int(float64(maxOut) * outputTypicalFraction)
			p.OutputWorst += maxOut
		}
	}

	computeCosts(&p)
	return p
}

func (e *Estimator) estimatePass1JCL(files []graph.FileInfo) PassEstimate {
	p := PassEstimate{
		Name:          "Pass 1 (JCL)",
		Model:         "sonnet",
		Deterministic: true,
	}
	maxOut := e.Config.Claude.Pass1MaxTokens

	for _, fi := range files {
		tok := estimateFileTokens(fi) + pass1PromptOverhead
		p.Requests++
		p.InputTokens += tok
		p.OutputTypical += int(float64(maxOut) * outputTypicalFraction)
		p.OutputWorst += maxOut
	}

	computeCosts(&p)
	return p
}

func (e *Estimator) estimatePass2(files []graph.FileInfo, cbIndex chunker.CopybookIndex) PassEstimate {
	p := PassEstimate{
		Name:          "Pass 2 (Deep Analysis)",
		Model:         "opus",
		Deterministic: true,
	}
	maxOut := e.Config.Claude.Pass2MaxTokens
	opts := chunker.Pass2ChunkOptions{
		TokenLimit:    e.Config.Ingest.Pass2TokenLimit,
		OverlapLines:  e.Config.Ingest.OverlapLines,
		CopybookIndex: cbIndex,
	}

	for _, fi := range files {
		chunks, err := chunker.ChunkFilePass2(fi, opts, e.Logger)
		if err != nil {
			e.Logger.Warn("estimate: pass2 chunk error", zap.String("file", fi.Path), zap.Error(err))
			p.Requests++
			tok := int(float64(fi.Size)/chunker.TokenEstimationRatio) + pass2PromptOverhead
			p.InputTokens += tok
			p.OutputTypical += int(float64(maxOut) * outputTypicalFraction)
			p.OutputWorst += maxOut
			continue
		}
		for _, c := range chunks {
			p.Requests++
			p.InputTokens += chunker.EstimateTokens(c.Content) + pass2PromptOverhead
			p.OutputTypical += int(float64(maxOut) * outputTypicalFraction)
			p.OutputWorst += maxOut
		}
	}

	computeCosts(&p)
	return p
}

func (e *Estimator) estimatePass3(cobolCount int) PassEstimate {
	if cobolCount == 0 {
		return PassEstimate{Name: "Pass 3 (Cross-Cutting)", Model: "opus", Deterministic: false}
	}
	batchSize := e.Config.Ingest.Pass3BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}
	requests := int(math.Ceil(float64(cobolCount) / float64(batchSize)))
	maxOut := e.Config.Claude.Pass3MaxTokens
	inputPerBatch := batchSize*pass3TokensPerProgram + pass3PromptOverhead
	totalInput := requests * inputPerBatch

	p := PassEstimate{
		Name:          "Pass 3 (Cross-Cutting)",
		Model:         "opus",
		Requests:      requests,
		InputTokens:   totalInput,
		OutputTypical: int(float64(maxOut*requests) * outputTypicalFraction),
		OutputWorst:   maxOut * requests,
		Deterministic: false,
		Notes:         "Request count exact; input size per batch estimated from program count",
	}
	computeCosts(&p)
	return p
}

func (e *Estimator) estimatePass4(cobolCount int) PassEstimate {
	if cobolCount == 0 {
		return PassEstimate{Name: "Pass 4 (Data Flow)", Model: "sonnet", Deterministic: false}
	}
	requests := int(math.Ceil(float64(cobolCount) * pass4CallPairRate))
	if requests < 1 {
		requests = 1
	}
	maxOut := e.Config.Claude.Pass4MaxTokens
	totalInput := requests * (pass4PromptOverhead + pass4InputPerPair)

	p := PassEstimate{
		Name:          "Pass 4 (Data Flow)",
		Model:         "sonnet",
		Requests:      requests,
		InputTokens:   totalInput,
		OutputTypical: int(float64(maxOut*requests) * outputTypicalFraction),
		OutputWorst:   maxOut * requests,
		Deterministic: false,
		Notes:         "Heuristic: ~25% of programs have LINKAGE/COMMAREA/file-flow call pairs",
	}
	computeCosts(&p)
	return p
}

func (e *Estimator) estimatePass5Repair(cobolCount int) PassEstimate {
	if cobolCount == 0 {
		return PassEstimate{Name: "Pass 5 (Repair)", Model: "opus", Deterministic: false}
	}
	requests := int(math.Ceil(float64(cobolCount) * pass5RepairRate))
	if requests < 1 {
		requests = 1
	}
	maxOut := e.Config.Claude.Pass2MaxTokens // repair uses pass2 budget
	totalInput := requests * (pass5RepairOverhead + e.Config.Ingest.Pass2TokenLimit)

	p := PassEstimate{
		Name:          "Pass 5 (Repair)",
		Model:         "opus",
		Requests:      requests,
		InputTokens:   totalInput,
		OutputTypical: int(float64(maxOut*requests) * outputTypicalFraction),
		OutputWorst:   maxOut * requests,
		Deterministic: false,
		Notes:         "Heuristic: ~35% of programs need relationship repair on large codebases",
	}
	computeCosts(&p)
	return p
}

func (e *Estimator) estimatePass5Annotations(cobolCount int) PassEstimate {
	if cobolCount == 0 {
		return PassEstimate{Name: "Pass 5 (Annotations)", Model: "opus", Deterministic: false}
	}
	requests := int(math.Ceil(float64(cobolCount) * pass5AnnotationRepairRate))
	if requests < 1 {
		requests = 1
	}
	maxOut := e.Config.Claude.Pass2MaxTokens // annotation repair uses pass2 budget (Opus)
	totalInput := requests * (pass5RepairOverhead + e.Config.Ingest.Pass2TokenLimit)

	p := PassEstimate{
		Name:          "Pass 5 (Annotations)",
		Model:         "opus",
		Requests:      requests,
		InputTokens:   totalInput,
		OutputTypical: int(float64(maxOut*requests) * outputTypicalFraction),
		OutputWorst:   maxOut * requests,
		Deterministic: false,
		Notes:         "Heuristic: ~20% of programs missing paragraph annotations after Pass 2",
	}
	computeCosts(&p)
	return p
}

func (e *Estimator) estimatePass5Verify(cobolCount int) PassEstimate {
	if cobolCount == 0 {
		return PassEstimate{Name: "Pass 5 (Verify)", Model: "sonnet", Deterministic: false}
	}
	deadRequests := int(math.Ceil(float64(cobolCount) * pass5DeadVerifyRate))
	requests := deadRequests + pass5DomainMergeCalls
	maxOut := e.Config.Claude.Pass1MaxTokens // verify uses sonnet budget
	// Dead verify: small program context (~1/4 of token limit) + overhead
	// Domain merge: small batch of domain names
	totalInput := deadRequests*(pass5VerifyOverhead+e.Config.Ingest.TokenLimit/4) +
		pass5DomainMergeCalls*(pass5VerifyOverhead+500)

	p := PassEstimate{
		Name:          "Pass 5 (Verify)",
		Model:         "sonnet",
		Requests:      requests,
		InputTokens:   totalInput,
		OutputTypical: int(float64(maxOut*requests) * outputTypicalFraction),
		OutputWorst:   maxOut * requests,
		Deterministic: false,
		Notes:         "Heuristic: ~15% dead verify + domain merge calls",
	}
	computeCosts(&p)
	return p
}

func (e *Estimator) estimateScanner(pendingCount int) ScannerEstimate {
	if pendingCount == 0 {
		return ScannerEstimate{}
	}
	requests := int(math.Ceil(float64(pendingCount) / float64(classifyBatchSize)))
	inputPerReq := classifySnippetTokens*classifyBatchSize + classifyPromptOverhead
	totalInput := requests * inputPerReq

	return ScannerEstimate{
		PendingFiles:   pendingCount,
		Requests:       requests,
		InputTokens:    totalInput,
		OutputTypical:  requests * int(float64(classifyMaxTokens)*outputTypicalFraction),
		OutputWorst:    requests * classifyMaxTokens * 3, // worst: 3 classification attempts
		InputCost:      anthropicInputCost("sonnet", totalInput),
		OutputCostLow:  anthropicOutputCost("sonnet", requests*int(float64(classifyMaxTokens)*outputTypicalFraction)),
		OutputCostHigh: anthropicOutputCost("sonnet", requests*classifyMaxTokens*3),
		CopilotCost:    copilotRequestCost("sonnet", requests),
	}
}

// computeCosts fills in Anthropic and Copilot pricing on a PassEstimate.
// Copilot cost includes the truncation retry multiplier since every retry
// (compression + token-doubling) is a separately billed premium request.
func computeCosts(p *PassEstimate) {
	p.InputCost = anthropicInputCost(p.Model, p.InputTokens)
	p.OutputCostLow = anthropicOutputCost(p.Model, p.OutputTypical)
	p.OutputCostHigh = anthropicOutputCost(p.Model, p.OutputWorst)
	// Apply retry multiplier: truncation retries resend the full prompt as new billed requests.
	billableRequests := int(math.Ceil(float64(p.Requests) * truncationRetryMultiplier))
	p.CopilotCost = copilotRequestCost(p.Model, billableRequests)
}

// estimateFileTokens reads the file to estimate tokens, falling back to file size on error.
func estimateFileTokens(fi graph.FileInfo) int {
	data, err := os.ReadFile(fi.Path)
	if err != nil {
		return int(float64(fi.Size) / chunker.TokenEstimationRatio)
	}
	return chunker.EstimateTokens(string(data))
}

