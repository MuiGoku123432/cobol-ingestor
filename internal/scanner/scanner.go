package scanner

import (
	"context"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// ScanResult holds the outcome of a filesystem scan.
type ScanResult struct {
	Files    []graph.FileInfo
	Duration time.Duration
	Errors   []error
}

// Scan walks rootDir, classifies source files, and computes SHA-256 hashes.
func Scan(ctx context.Context, rootDir string, logger *zap.Logger) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{}

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

		ft, ok := classifyFile(d.Name())
		if !ok {
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
			Type:      ft,
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

	logger.Info("scan complete",
		zap.Int("files", len(result.Files)),
		zap.Int("errors", len(result.Errors)),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

func classifyFile(name string) (graph.FileType, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".cbl", ".cob":
		return graph.FileTypeCOBOL, true
	case ".cpy", ".cpb":
		return graph.FileTypeCopybook, true
	case ".jcl":
		return graph.FileTypeJCL, true
	default:
		return "", false
	}
}

// hashAndCountLines computes SHA-256 and counts newlines in a single pass.
func hashAndCountLines(path string) (string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	reader := io.TeeReader(f, h)
	scanner := bufio.NewScanner(reader)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), lines, nil
}
