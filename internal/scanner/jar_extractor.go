package scanner

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// maxTotalExtraction is the aggregate extraction size limit per top-level JAR (500MB).
const maxTotalExtraction = 500 * 1024 * 1024

// JAREntry represents a single analyzable file extracted from a JAR archive.
type JAREntry struct {
	FileInfo  graph.FileInfo // Virtual path: "path/to/app.jar!/com/example/Foo.java"
	Content   []byte         // In-memory text (raw or disassembled)
	JARPath   string         // Physical JAR path on disk
	JARHash   string         // SHA-256 of the whole JAR
	EntryPath string         // Internal ZIP entry path
}

// isNestedArchive returns true if the entry name has a JAR/WAR/EAR/ZIP extension.
func isNestedArchive(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jar", ".war", ".ear", ".zip":
		return true
	}
	return false
}

// ExtractJAR opens a JAR via archive/zip, computes a JAR-level SHA-256, iterates
// entries, and extracts analyzable ones in-memory. .class files are disassembled
// using javap (falling back to Go-native parsing if unavailable). Nested archives
// are recursively extracted up to maxDepth levels.
func ExtractJAR(ctx context.Context, jarPath string, javapPath string, logger *zap.Logger, maxDepth int, maxWorkers int) ([]JAREntry, error) {
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	jarHash, err := hashFile(jarPath)
	if err != nil {
		return nil, fmt.Errorf("hashing JAR %s: %w", jarPath, err)
	}

	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, fmt.Errorf("opening JAR %s: %w", jarPath, err)
	}
	defer r.Close()

	var totalExtracted int64
	entries, err := extractArchiveFromReader(ctx, r.File, jarPath, jarHash, javapPath, logger, 0, maxDepth, &totalExtracted, maxWorkers)
	if err != nil {
		return nil, err
	}

	logger.Info("extracted JAR entries",
		zap.String("jar", jarPath),
		zap.Int("entries", len(entries)),
		zap.String("hash", jarHash),
		zap.Int64("totalBytes", totalExtracted),
	)

	return entries, nil
}

// extractArchiveFromReader is the core recursive function that processes zip entries.
// Virtual paths chain with "!/": outer.jar!/lib/inner.jar!/Foo.class
func extractArchiveFromReader(
	ctx context.Context,
	files []*zip.File,
	basePath string,
	jarHash string,
	javapPath string,
	logger *zap.Logger,
	depth int,
	maxDepth int,
	totalExtracted *int64,
	maxWorkers int,
) ([]JAREntry, error) {
	var mu sync.Mutex
	var entries []JAREntry
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	var firstErr atomic.Value

	for _, f := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errVal := firstErr.Load(); errVal != nil {
			return nil, errVal.(error)
		}

		if f.FileInfo().IsDir() {
			continue
		}

		// Nested archives recurse inline (not parallelized) to avoid goroutine explosion
		if isNestedArchive(f.Name) {
			if depth >= maxDepth {
				logger.Debug("skipping nested archive (depth limit reached)",
					zap.String("archive", basePath),
					zap.String("entry", f.Name),
					zap.Int("depth", depth),
					zap.Int("maxDepth", maxDepth),
				)
				continue
			}

			data, err := readZipEntry(f)
			if err != nil {
				logger.Warn("failed to read nested archive entry",
					zap.String("archive", basePath),
					zap.String("entry", f.Name),
					zap.Error(err),
				)
				continue
			}

			if atomic.AddInt64(totalExtracted, int64(len(data))) > maxTotalExtraction {
				logger.Warn("total extraction size limit reached, skipping remaining nested archives",
					zap.String("archive", basePath),
					zap.String("entry", f.Name),
				)
				continue
			}

			nestedReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				logger.Warn("corrupt nested archive (skipping)",
					zap.String("archive", basePath),
					zap.String("entry", f.Name),
					zap.Error(err),
				)
				continue
			}

			nestedPath := basePath + "!/" + f.Name
			nestedEntries, err := extractArchiveFromReader(
				ctx, nestedReader.File, nestedPath, jarHash, javapPath, logger,
				depth+1, maxDepth, totalExtracted, maxWorkers,
			)
			if err != nil {
				return nil, err
			}
			mu.Lock()
			entries = append(entries, nestedEntries...)
			mu.Unlock()
			continue
		}

		if !isAnalyzableEntry(f.Name) {
			continue
		}

		// Parallelize leaf entry processing
		wg.Add(1)
		sem <- struct{}{}
		go func(zf *zip.File) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			data, err := readZipEntry(zf)
			if err != nil {
				logger.Warn("failed to read JAR entry",
					zap.String("archive", basePath),
					zap.String("entry", zf.Name),
					zap.Error(err),
				)
				return
			}

			if atomic.AddInt64(totalExtracted, int64(len(data))) > maxTotalExtraction {
				logger.Warn("total extraction size limit reached, skipping entry",
					zap.String("archive", basePath),
					zap.String("entry", zf.Name),
				)
				return
			}

			var content []byte
			ext := strings.ToLower(filepath.Ext(zf.Name))

			if ext == ".class" {
				text, disErr := disassembleClass(data, zf.Name, javapPath)
				if disErr != nil {
					classInfo, parseErr := ParseClassFile(data)
					if parseErr != nil {
						logger.Warn("failed to parse .class entry (skipping)",
							zap.String("archive", basePath),
							zap.String("entry", zf.Name),
							zap.Error(parseErr),
						)
						return
					}
					content = []byte(classInfo.FormatAsText())
				} else {
					content = []byte(text)
				}
			} else {
				content = data
			}

			virtualPath := basePath + "!/" + zf.Name
			lineCount := countLines(content)

			entry := JAREntry{
				FileInfo: graph.FileInfo{
					Path:      virtualPath,
					Type:      graph.FileTypeBW,
					Hash:      jarHash,
					Size:      int64(len(content)),
					LineCount: lineCount,
				},
				Content:   content,
				JARPath:   basePath,
				JARHash:   jarHash,
				EntryPath: zf.Name,
			}

			mu.Lock()
			entries = append(entries, entry)
			mu.Unlock()
		}(f)
	}

	wg.Wait()

	if errVal := firstErr.Load(); errVal != nil {
		return nil, errVal.(error)
	}

	return entries, nil
}

// isAnalyzableEntry returns true if the ZIP entry should be extracted for analysis.
func isAnalyzableEntry(name string) bool {
	// Always include MANIFEST.MF
	if strings.HasSuffix(strings.ToUpper(name), "MANIFEST.MF") {
		return true
	}

	ext := strings.ToLower(filepath.Ext(name))

	// Whitelist of analyzable extensions
	switch ext {
	case ".java", ".xml", ".properties", ".txt", ".md",
		".json", ".yml", ".yaml", ".cfg", ".conf", ".class":
		return true
	}

	// Blacklist (explicit reject for clarity)
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".bmp",
		".so", ".dll", ".dylib":
		return false
	}

	return false
}

// disassembleClass writes .class bytes to a temp file, runs javap -p, and returns the output.
func disassembleClass(classBytes []byte, className string, javapPath string) (string, error) {
	if javapPath == "" {
		return "", fmt.Errorf("javap not available")
	}

	tmpFile, err := os.CreateTemp("", "cobol-graph-*.class")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(classBytes); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("writing temp class file: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command(javapPath, "-p", tmpPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("javap failed for %s: %w (stderr: %s)", className, err, stderr.String())
	}

	return stdout.String(), nil
}

// hashFile computes the SHA-256 of an entire file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readZipEntry reads the full contents of a ZIP file entry into memory.
func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// Limit read to 50MB to avoid OOM on malicious entries
	const maxSize = 50 * 1024 * 1024
	lr := io.LimitReader(rc, maxSize)
	return io.ReadAll(lr)
}

// countLines counts newline characters in content.
func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	// If last byte isn't a newline, count that partial line
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}
