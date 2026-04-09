// Package tokencount provides token counting strategies for LLM cost estimation.
// It is used exclusively by the estimate package (--estimate mode) and is not
// in the hot path of real pipeline execution.
package tokencount

import (
	"cobol-ingestor/internal/chunker"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// Counter counts the tokens in a string.
type Counter interface {
	Count(content string) int
}

// HeuristicCounter estimates tokens using the chars-per-token ratio from chunker.
// This is the default: fast, no network dependency.
type HeuristicCounter struct{}

func (HeuristicCounter) Count(content string) int {
	return chunker.EstimateTokens(content)
}

// BPECounter counts tokens using the cl100k_base BPE encoding (GPT-4's tokenizer).
// Claude uses a proprietary tokenizer not available as a standalone library; cl100k_base
// is a close approximation, typically within ~10-15% for code and prose.
type BPECounter struct {
	enc *tiktoken.Tiktoken
}

// NewBPECounter initializes a BPECounter using cl100k_base encoding.
// The BPE vocabulary (~1.5MB) is downloaded on first use and cached in
// TIKTOKEN_CACHE_DIR (default: ~/.tiktoken_cache).
func NewBPECounter() (*BPECounter, error) {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, err
	}
	return &BPECounter{enc: enc}, nil
}

// Count returns the number of tokens in content using BPE encoding.
func (b *BPECounter) Count(content string) int {
	return len(b.enc.Encode(content, nil, nil))
}

// CountOrHeuristic calls counter.Count if counter is non-nil, otherwise falls
// back to chunker.EstimateTokens. Use this in estimators to avoid nil checks
// at every call site.
func CountOrHeuristic(counter Counter, content string) int {
	if counter != nil {
		return counter.Count(content)
	}
	return chunker.EstimateTokens(content)
}

// MethodLabel returns a human-readable description of the counting method.
func MethodLabel(counter Counter) string {
	if counter == nil {
		return "heuristic (chars/3.2) — use --precise for BPE-based counts"
	}
	return "BPE (cl100k_base) — Claude's actual tokenizer may differ ~10-15%"
}
