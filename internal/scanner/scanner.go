package scanner

import (
	"bufio"
	"context"
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

// ScanOptions controls optional scanning behavior.
type ScanOptions struct {
	ContentDetect bool // enable .txt file detection
}

// ScanResult holds the outcome of a filesystem scan.
type ScanResult struct {
	Files    []graph.FileInfo
	Snippets map[string][]string // path -> first N lines (for pending files)
	Duration time.Duration
	Errors   []error
}

// snippetMaxLines is the number of lines captured for content-based classification.
const snippetMaxLines = 50

// Scan walks rootDir, classifies source files, and computes SHA-256 hashes.
// Pass ScanOptions to enable content-based detection of .txt files.
func Scan(ctx context.Context, rootDir string, logger *zap.Logger, opts ...ScanOptions) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{}

	var opt ScanOptions
	if len(opts) > 0 {
		opt = opts[0]
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

		name := d.Name()

		// Fast path: known extension
		if ft, ok := classifyFile(name); ok {
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
		}

		// Content detection path for unrecognized extensions
		if !opt.ContentDetect {
			return nil
		}

		// Try double-extension: PROG.CBL.txt -> COBOL
		if ft, ok := classifyByDoubleExt(name); ok {
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
		}

		// Only .txt files get content-based classification
		if strings.ToLower(filepath.Ext(name)) != ".txt" {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("stat %s: %w", path, err))
			return nil
		}
		hash, lineCount, snippet, err := hashCountAndSnippet(path, snippetMaxLines)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("hash %s: %w", path, err))
			return nil
		}
		result.Files = append(result.Files, graph.FileInfo{
			Path:      path,
			Type:      graph.FileTypePending,
			Hash:      hash,
			Size:      info.Size(),
			LineCount: lineCount,
		})
		if result.Snippets == nil {
			result.Snippets = make(map[string][]string)
		}
		result.Snippets[path] = snippet

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", rootDir, err)
	}

	result.Duration = time.Since(start)

	logger.Info("scan complete",
		zap.Int("files", len(result.Files)),
		zap.Int("pending", len(result.Snippets)),
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

// classifyByDoubleExt strips the outer extension and tries classifyFile on the remainder.
// Catches patterns like PROG.CBL.txt, COPY.CPY.txt, JOB.JCL.txt.
func classifyByDoubleExt(name string) (graph.FileType, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return "", false
	}
	inner := name[:len(name)-len(ext)]
	if inner == "" {
		return "", false
	}
	return classifyFile(inner)
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

// hashCountAndSnippet computes SHA-256, counts lines, and captures the first maxLines
// in a single pass through the file.
func hashCountAndSnippet(path string, maxLines int) (string, int, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, nil, err
	}
	defer f.Close()

	h := sha256.New()
	reader := io.TeeReader(f, h)
	sc := bufio.NewScanner(reader)
	lines := 0
	var snippet []string
	for sc.Scan() {
		lines++
		if lines <= maxLines {
			snippet = append(snippet, sc.Text())
		}
	}
	if err := sc.Err(); err != nil {
		return "", 0, nil, err
	}
	return hex.EncodeToString(h.Sum(nil)), lines, snippet, nil
}
