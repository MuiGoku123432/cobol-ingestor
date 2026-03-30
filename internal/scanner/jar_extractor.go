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

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// JAREntry represents a single analyzable file extracted from a JAR archive.
type JAREntry struct {
	FileInfo  graph.FileInfo // Virtual path: "path/to/app.jar!/com/example/Foo.java"
	Content   []byte         // In-memory text (raw or disassembled)
	JARPath   string         // Physical JAR path on disk
	JARHash   string         // SHA-256 of the whole JAR
	EntryPath string         // Internal ZIP entry path
}

// ExtractJAR opens a JAR via archive/zip, computes a JAR-level SHA-256, iterates
// entries, and extracts analyzable ones in-memory. .class files are disassembled
// using javap (falling back to Go-native parsing if unavailable).
func ExtractJAR(ctx context.Context, jarPath string, javapPath string, logger *zap.Logger) ([]JAREntry, error) {
	jarHash, err := hashFile(jarPath)
	if err != nil {
		return nil, fmt.Errorf("hashing JAR %s: %w", jarPath, err)
	}

	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, fmt.Errorf("opening JAR %s: %w", jarPath, err)
	}
	defer r.Close()

	var entries []JAREntry
	for _, f := range r.File {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if f.FileInfo().IsDir() {
			continue
		}

		if !isAnalyzableEntry(f.Name) {
			// Log warning for nested archives
			ext := strings.ToLower(filepath.Ext(f.Name))
			if ext == ".jar" || ext == ".war" || ext == ".ear" {
				logger.Warn("skipping nested archive inside JAR",
					zap.String("jar", jarPath),
					zap.String("entry", f.Name),
				)
			}
			continue
		}

		data, err := readZipEntry(f)
		if err != nil {
			logger.Warn("failed to read JAR entry",
				zap.String("jar", jarPath),
				zap.String("entry", f.Name),
				zap.Error(err),
			)
			continue
		}

		var content []byte
		ext := strings.ToLower(filepath.Ext(f.Name))

		if ext == ".class" {
			text, disErr := disassembleClass(data, f.Name, javapPath)
			if disErr != nil {
				// Fall back to Go-native parser
				classInfo, parseErr := ParseClassFile(data)
				if parseErr != nil {
					logger.Warn("failed to parse .class entry (skipping)",
						zap.String("jar", jarPath),
						zap.String("entry", f.Name),
						zap.Error(parseErr),
					)
					continue
				}
				content = []byte(classInfo.FormatAsText())
			} else {
				content = []byte(text)
			}
		} else {
			content = data
		}

		virtualPath := jarPath + "!/" + f.Name
		lineCount := countLines(content)

		entries = append(entries, JAREntry{
			FileInfo: graph.FileInfo{
				Path:      virtualPath,
				Type:      graph.FileTypeBW,
				Hash:      jarHash,
				Size:      int64(len(content)),
				LineCount: lineCount,
			},
			Content:   content,
			JARPath:   jarPath,
			JARHash:   jarHash,
			EntryPath: f.Name,
		})
	}

	logger.Info("extracted JAR entries",
		zap.String("jar", jarPath),
		zap.Int("entries", len(entries)),
		zap.String("hash", jarHash),
	)

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
		".jar", ".war", ".ear", ".zip",
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
