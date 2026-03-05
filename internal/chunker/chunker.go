package chunker

import (
	"fmt"
	"os"

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
		},
	}, nil
}

// EstimateTokens provides a rough token count (chars / 4).
func EstimateTokens(content string) int {
	return len(content) / 4
}
