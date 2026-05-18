package targetstack

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// noiseDirs lists directories that should always be skipped.
var noiseDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true,
	"build": true, "dist": true, "__pycache__": true, ".idea": true,
	".vscode": true, "out": true, "bin": true, ".gradle": true,
	".mvn": true, "coverage": true, ".nyc_output": true, "tmp": true,
}

// testIndicators identifies test files by path pattern.
var testIndicators = []string{"_test.", "test_", "/test/", "/tests/", "/spec/", "__test__", ".spec.", ".test."}

// buildIndicators identifies build/config files by extension.
var buildIndicators = []string{".gradle", "pom.xml", "Makefile", "Dockerfile", ".dockerfile",
	"docker-compose", ".sh", ".bat", ".cmd", "Jenkinsfile", ".github"}

// schemaIndicators identifies schema/contract files by extension.
var schemaExt = map[string]bool{
	".graphql": true, ".proto": true, ".wsdl": true, ".xsd": true, ".avsc": true,
}

// configExt identifies config/metadata files.
var configExt = map[string]bool{
	".yaml": true, ".yml": true, ".json": true, ".xml": true, ".toml": true,
	".ini": true, ".properties": true, ".env": true,
}

// langByExt maps file extensions to language names.
var langByExt = map[string]string{
	".java": "Java", ".kt": "Kotlin", ".scala": "Scala",
	".cs": "C#", ".vb": "VB.NET",
	".py": "Python",
	".ts": "TypeScript", ".tsx": "TypeScript", ".js": "JavaScript", ".jsx": "JavaScript",
	".go": "Go",
	".rb": "Ruby",
	".php": "PHP",
	".rs": "Rust",
	".swift": "Swift",
	".cpp": "C++", ".cc": "C++", ".c": "C",
	".xml": "XML", ".yaml": "YAML", ".yml": "YAML", ".json": "JSON",
	".graphql": "GraphQL", ".proto": "Protobuf",
}

// Scan walks localPath and collects files matching the given extensions.
// It respects .cobol-graph-ignore if present at the repo root.
func Scan(localPath string, extensions []string, logger *zap.Logger) (*ScanResult, error) {
	extSet := normalizeExtensions(extensions)
	ignoreSet := loadIgnoreFile(localPath)

	result := &ScanResult{
		LocalPath: localPath,
		ByLang:    make(map[string]int),
	}

	err := filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}

		rel, _ := filepath.Rel(localPath, path)

		// Skip noise/ignored directories
		if d.IsDir() {
			name := d.Name()
			if noiseDirs[name] || ignoreSet[name] || strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !extSet[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			logger.Warn("hashing file", zap.String("path", path), zap.Error(err))
			return nil
		}

		lang := langByExt[ext]
		cat := classify(rel, ext)

		f := ScannedFile{
			Path:     path,
			RelPath:  rel,
			Hash:     hash,
			Size:     info.Size(),
			Language: lang,
			Category: cat,
		}
		result.Files = append(result.Files, f)
		if lang != "" {
			result.ByLang[lang]++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", localPath, err)
	}

	return result, nil
}

// hashFile computes a SHA-256 hex digest of a file's contents.
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
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// classify determines the FileCategory of a file based on path patterns and extension.
func classify(relPath, ext string) FileCategory {
	lower := strings.ToLower(relPath)
	for _, ind := range testIndicators {
		if strings.Contains(lower, ind) {
			return FileCategoryTest
		}
	}
	if schemaExt[ext] {
		return FileCategorySchema
	}
	for _, ind := range buildIndicators {
		if strings.Contains(lower, strings.ToLower(ind)) {
			return FileCategoryBuild
		}
	}
	if configExt[ext] {
		return FileCategoryConfig
	}
	if _, ok := langByExt[ext]; ok {
		return FileCategorySource
	}
	return FileCategoryOther
}

// normalizeExtensions converts a list of extensions to a lowercase set.
func normalizeExtensions(exts []string) map[string]bool {
	set := make(map[string]bool, len(exts))
	for _, e := range exts {
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		set[strings.ToLower(e)] = true
	}
	return set
}

// loadIgnoreFile reads a .cobol-graph-ignore file at the repo root and returns
// a set of directory/file names to skip.
func loadIgnoreFile(repoRoot string) map[string]bool {
	set := make(map[string]bool)
	data, err := os.ReadFile(filepath.Join(repoRoot, ".cobol-graph-ignore"))
	if err != nil {
		return set
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			set[line] = true
		}
	}
	return set
}
