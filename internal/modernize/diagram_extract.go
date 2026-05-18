package modernize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cobol-ingestor/internal/diagram"
)

// DiagramResult holds the result of rendering a single Mermaid block.
type DiagramResult struct {
	FilePath string
	SvgData  string
	Error    string // non-empty if render failed
}

// ExtractAndRenderMermaidBlocks scans text for ```mermaid fenced code blocks,
// renders each to SVG, writes to outputDir, and returns diagram results.
func ExtractAndRenderMermaidBlocks(text, outputDir string) []DiagramResult {
	blocks := extractMermaidBlocks(text)
	if len(blocks) == 0 {
		return nil
	}

	if outputDir != "" {
		_ = os.MkdirAll(outputDir, 0700)
	}

	timestamp := time.Now().Format("20060102-150405")
	var results []DiagramResult

	for i, code := range blocks {
		svg, err := diagram.RenderMermaidSVG(code, "github-dark")
		if err != nil {
			results = append(results, DiagramResult{Error: err.Error()})
			continue
		}

		var filePath string
		if outputDir != "" {
			fileName := fmt.Sprintf("diagram-%s-%d.svg", timestamp, i)
			filePath = filepath.Join(outputDir, fileName)
			_ = os.WriteFile(filePath, []byte(svg), 0644)
		}

		results = append(results, DiagramResult{
			FilePath: filePath,
			SvgData:  svg,
		})
	}

	return results
}

// extractMermaidBlocks finds all ```mermaid ... ``` fenced code blocks in text.
func extractMermaidBlocks(text string) []string {
	var blocks []string
	remaining := text

	for {
		// Find opening fence
		startMarker := "```mermaid"
		idx := strings.Index(remaining, startMarker)
		if idx == -1 {
			break
		}

		// Skip past the marker and any trailing whitespace/newline
		afterMarker := remaining[idx+len(startMarker):]
		if len(afterMarker) > 0 && afterMarker[0] == '\n' {
			afterMarker = afterMarker[1:]
		} else if len(afterMarker) > 1 && afterMarker[0] == '\r' && afterMarker[1] == '\n' {
			afterMarker = afterMarker[2:]
		}

		// Find closing fence
		endIdx := strings.Index(afterMarker, "```")
		if endIdx == -1 {
			break
		}

		code := strings.TrimSpace(afterMarker[:endIdx])
		if code != "" {
			blocks = append(blocks, code)
		}

		remaining = afterMarker[endIdx+3:]
	}

	return blocks
}
