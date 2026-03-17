package strategy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sectionDef maps a section marker to its output filename.
type sectionDef struct {
	Marker   string
	Filename string
}

var sections = []sectionDef{
	{Marker: "<!-- SECTION: executive-summary -->", Filename: "01-executive-summary.md"},
	{Marker: "<!-- SECTION: strategy-overview -->", Filename: "02-strategy-overview.md"},
	{Marker: "<!-- SECTION: codebase-assessment -->", Filename: "03-codebase-assessment.md"},
	{Marker: "<!-- SECTION: wave-plan -->", Filename: "04-wave-plan.md"},
	{Marker: "<!-- SECTION: risk-assessment -->", Filename: "05-risk-assessment.md"},
	{Marker: "<!-- SECTION: effort-estimates -->", Filename: "06-effort-estimates.md"},
	{Marker: "<!-- SECTION: data-migration -->", Filename: "07-data-migration.md"},
	{Marker: "<!-- SECTION: integration-strategy -->", Filename: "08-integration-strategy.md"},
	{Marker: "<!-- SECTION: testing-strategy -->", Filename: "09-testing-strategy.md"},
	{Marker: "<!-- SECTION: timeline-roadmap -->", Filename: "10-timeline-roadmap.md"},
}

// GenerateStrategyDocs splits the coordinator synthesis into separate files.
// Returns the list of written file paths.
func GenerateStrategyDocs(plan *StrategyPlan, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	synthesis := plan.Synthesis

	// Try to split by section markers
	fileParts := splitBySections(synthesis)
	if fileParts == nil {
		// Fallback: write entire synthesis as single file
		path := filepath.Join(outputDir, "migration-strategy.md")
		if err := os.WriteFile(path, []byte(synthesis), 0644); err != nil {
			return nil, fmt.Errorf("writing strategy file: %w", err)
		}
		return []string{path}, nil
	}

	// Write each section as a separate file
	var written []string
	for _, fp := range fileParts {
		content := strings.TrimSpace(fp.Content)
		if content == "" {
			continue
		}
		path := filepath.Join(outputDir, fp.Filename)
		if err := os.WriteFile(path, []byte(content+"\n"), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", fp.Filename, err)
		}
		written = append(written, path)
	}

	// Generate README with TOC
	readme := generateReadme(written, outputDir)
	readmePath := filepath.Join(outputDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return nil, fmt.Errorf("writing README: %w", err)
	}
	written = append([]string{readmePath}, written...)

	return written, nil
}

type filePart struct {
	Filename string
	Content  string
}

func splitBySections(synthesis string) []filePart {
	// Check if at least 3 section markers are present
	markerCount := 0
	for _, s := range sections {
		if strings.Contains(synthesis, s.Marker) {
			markerCount++
		}
	}
	if markerCount < 3 {
		return nil
	}

	var parts []filePart
	remaining := synthesis

	for i, s := range sections {
		idx := strings.Index(remaining, s.Marker)
		if idx == -1 {
			continue
		}

		// Content starts after the marker
		contentStart := idx + len(s.Marker)

		// Find the end: next section marker or end of string
		var contentEnd int
		found := false
		for j := i + 1; j < len(sections); j++ {
			nextIdx := strings.Index(remaining[contentStart:], sections[j].Marker)
			if nextIdx != -1 {
				contentEnd = contentStart + nextIdx
				found = true
				break
			}
		}
		if !found {
			contentEnd = len(remaining)
		}

		content := remaining[contentStart:contentEnd]
		parts = append(parts, filePart{
			Filename: s.Filename,
			Content:  content,
		})
	}

	if len(parts) == 0 {
		return nil
	}
	return parts
}

func generateReadme(files []string, outputDir string) string {
	var sb strings.Builder
	sb.WriteString("# Migration Strategy\n\n")
	sb.WriteString("Generated migration strategy documents for the COBOL codebase.\n\n")
	sb.WriteString("## Documents\n\n")

	for _, f := range files {
		name := filepath.Base(f)
		if name == "README.md" {
			continue
		}
		// Convert filename to title: "01-executive-summary.md" -> "Executive Summary"
		title := filenameToTitle(name)
		sb.WriteString(fmt.Sprintf("- [%s](%s)\n", title, name))
	}

	return sb.String()
}

func filenameToTitle(name string) string {
	// Strip extension
	name = strings.TrimSuffix(name, ".md")
	// Strip leading number prefix (e.g., "01-")
	if len(name) > 3 && name[2] == '-' {
		name = name[3:]
	}
	// Replace dashes with spaces and title case
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
