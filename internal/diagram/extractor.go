package diagram

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ExtractText converts a diagram file's binary/compressed content into
// structured text that an LLM can analyze. Dispatches to format-specific
// extractors based on file extension.
func ExtractText(path string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".drawio":
		return extractDrawIO(content)
	case ".vsdx":
		return extractVSDX(content)
	case ".svg":
		return extractSVG(content)
	case ".puml", ".plantuml":
		return extractPlantUML(content)
	default:
		return "", fmt.Errorf("unsupported diagram format: %s", ext)
	}
}

// IsDiagramExtension returns true if the extension is a supported diagram format.
func IsDiagramExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".drawio", ".vsdx", ".svg", ".puml", ".plantuml":
		return true
	default:
		return false
	}
}
