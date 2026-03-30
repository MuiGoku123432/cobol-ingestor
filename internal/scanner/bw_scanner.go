package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// ScanBWResult extends ScanResult with pre-extracted JAR content.
type ScanBWResult struct {
	*ScanResult
	JARContents map[string][]byte // virtual path -> in-memory content
}

// DefaultBWExtensions lists the default file extensions for Businessware scanning.
var DefaultBWExtensions = []string{".java", ".md", ".bw", ".txt", ".xml", ".vsdx", ".drawio", ".svg", ".puml", ".plantuml", ".jar"}

// ScanBW walks rootDir and discovers Businessware files matching the given extensions.
// JAR files are extracted in-memory and their entries are added as virtual files.
func ScanBW(ctx context.Context, rootDir string, extensions []string, logger *zap.Logger) (*ScanBWResult, error) {
	start := time.Now()
	result := &ScanBWResult{
		ScanResult:  &ScanResult{},
		JARContents: make(map[string][]byte),
	}

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

	// Auto-detect javap once
	javapPath, _ := exec.LookPath("javap")
	if javapPath == "" {
		logger.Info("javap not found on PATH, will use Go-native .class parser for JAR entries")
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

		// Handle JAR files: extract entries as virtual files
		if ext == ".jar" {
			entries, extractErr := ExtractJAR(ctx, path, javapPath, logger)
			if extractErr != nil {
				logger.Error("failed to extract JAR (skipping)",
					zap.String("jar", path),
					zap.Error(extractErr),
				)
				result.Errors = append(result.Errors, fmt.Errorf("extract JAR %s: %w", path, extractErr))
				return nil
			}
			for _, entry := range entries {
				result.Files = append(result.Files, entry.FileInfo)
				result.JARContents[entry.FileInfo.Path] = entry.Content
			}
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
