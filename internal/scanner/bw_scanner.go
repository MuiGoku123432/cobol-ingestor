package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
var DefaultBWExtensions = []string{".java", ".md", ".bw", ".txt", ".xml", ".vsdx", ".drawio", ".svg", ".puml", ".plantuml", ".jar", ".war", ".ear"}

// ScanBW walks rootDir and discovers Businessware files matching the given extensions.
// JAR files are extracted in-memory and their entries are added as virtual files.
func ScanBW(ctx context.Context, rootDir string, extensions []string, logger *zap.Logger, maxDepth int, maxWorkers int) (*ScanBWResult, error) {
	if maxWorkers < 1 {
		maxWorkers = 1
	}

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

	// Phase 1: Walk and collect regular files + JAR paths
	var jarPaths []string
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("walk %s: %w", path, err))
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

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

		// Collect JAR/WAR/EAR paths for concurrent extraction
		if ext == ".jar" || ext == ".war" || ext == ".ear" {
			jarPaths = append(jarPaths, path)
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

	// Phase 2: Extract JARs concurrently
	if len(jarPaths) > 0 {
		type jarResult struct {
			entries []JAREntry
			err     error
			path    string
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxWorkers)

		var jarResults []jarResult

		for _, jp := range jarPaths {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(p string) {
				defer wg.Done()
				defer func() { <-sem }()
				entries, err := ExtractJAR(ctx, p, javapPath, logger, maxDepth, maxWorkers)
				mu.Lock()
				jarResults = append(jarResults, jarResult{entries: entries, err: err, path: p})
				mu.Unlock()
			}(jp)
		}
		wg.Wait()

		for _, jr := range jarResults {
			if jr.err != nil {
				logger.Error("failed to extract JAR (skipping)",
					zap.String("jar", jr.path),
					zap.Error(jr.err),
				)
				result.Errors = append(result.Errors, fmt.Errorf("extract JAR %s: %w", jr.path, jr.err))
				continue
			}
			for _, entry := range jr.entries {
				result.Files = append(result.Files, entry.FileInfo)
				result.JARContents[entry.FileInfo.Path] = entry.Content
			}
		}
	}

	result.Duration = time.Since(start)

	logger.Info("BW scan complete",
		zap.Int("files", len(result.Files)),
		zap.Int("errors", len(result.Errors)),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}
