package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// DefaultBWExtensions lists the default file extensions for Businessware scanning.
var DefaultBWExtensions = []string{".java", ".md", ".bw", ".txt", ".xml"}

// ScanBW walks rootDir and discovers Businessware files matching the given extensions.
func ScanBW(ctx context.Context, rootDir string, extensions []string, logger *zap.Logger) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{}

	if len(extensions) == 0 {
		extensions = DefaultBWExtensions
	}

	extSet := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extSet[strings.ToLower(ext)] = true
	}

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("walk %s: %w", path, err))
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !extSet[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("stat %s: %w", path, err))
			return nil
		}

		hash, lineCount, err := hashAndCountLines(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("hash %s: %w", path, err))
			return nil
		}

		result.Files = append(result.Files, graph.FileInfo{
			Path:      path,
			Type:      graph.FileTypeBW,
			Hash:      hash,
			Size:      info.Size(),
			LineCount: lineCount,
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", rootDir, err)
	}

	result.Duration = time.Since(start)

	logger.Info("BW scan complete",
		zap.Int("files", len(result.Files)),
		zap.Int("errors", len(result.Errors)),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}
