package static

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cobol-ingestor/internal/graph"
)

// dataItem represents a parsed COBOL data item from the DATA DIVISION.
type dataItem struct {
	Name     string
	Level    int
	Line     int
	Copybook string // from *>> COPY markers
}

// levelRegex matches COBOL data item declarations: level-number name.
// Handles levels 01-88, 66, 77.
var levelRegex = regexp.MustCompile(`(?i)^\s+(\d{2})\s+([A-Za-z][A-Za-z0-9_-]*)\b`)

// copyInlinedBeginRegex matches the copybook inline marker.
var copyInlinedBeginRegex = regexp.MustCompile(`\*>>\s*COPY\s+([A-Za-z0-9_-]+(?:\s+[A-Za-z0-9_-]+)?)\s+INLINED\s+BEGIN`)

// copyInlinedEndRegex matches the copybook inline end marker.
var copyInlinedEndRegex = regexp.MustCompile(`\*>>\s*COPY\s+[A-Za-z0-9_-]+(?:\s+[A-Za-z0-9_-]+)?\s+INLINED\s+END`)

// ExtractDataHierarchy parses the DATA DIVISION to extract CHILD_OF relationships
// deterministically from COBOL level numbers.
//
// COBOL rule: an item at level N is a child of the nearest preceding item at a level < N.
// Special levels: 66 (RENAMES), 77 (independent), 88 (condition) are excluded from hierarchy.
func ExtractDataHierarchy(dataDivision, programID string) []graph.Relationship {
	if dataDivision == "" {
		return nil
	}

	lines := strings.Split(dataDivision, "\n")

	// Parse all data items
	var items []dataItem
	currentCopybook := ""

	for _, line := range lines {
		// Track copybook context
		if m := copyInlinedBeginRegex.FindStringSubmatch(line); len(m) >= 2 {
			currentCopybook = m[1]
			continue
		}
		if copyInlinedEndRegex.MatchString(line) {
			currentCopybook = ""
			continue
		}

		m := levelRegex.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}

		level, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}

		name := strings.ToUpper(m[2])

		// Skip special levels and FILLER
		if level == 66 || level == 77 || level == 88 {
			continue
		}
		if name == "FILLER" {
			continue
		}

		items = append(items, dataItem{
			Name:     name,
			Level:    level,
			Copybook: currentCopybook,
		})
	}

	if len(items) == 0 {
		return nil
	}

	// Build CHILD_OF relationships using a stack-based approach.
	// Maintain a stack of ancestor items. For each new item:
	// - Pop items with level >= current level
	// - The top of the stack is the parent
	// - Push the current item
	var relationships []graph.Relationship
	type stackEntry struct {
		name  string
		level int
	}
	var stack []stackEntry

	for _, item := range items {
		// Pop items with level >= current
		for len(stack) > 0 && stack[len(stack)-1].level >= item.Level {
			stack = stack[:len(stack)-1]
		}

		// If stack is non-empty, the top is our parent
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			childFQN := fmt.Sprintf("%s.%02d.%s", programID, item.Level, item.Name)
			parentFQN := fmt.Sprintf("%s.%02d.%s", programID, parent.level, parent.name)

			relationships = append(relationships, graph.Relationship{
				Type:      graph.RelChildOf,
				FromLabel: "DataItem",
				FromKey:   childFQN,
				ToLabel:   "DataItem",
				ToKey:     parentFQN,
				Properties: map[string]any{
					"source": "static_parser",
				},
			})
		}

		// Push current item
		stack = append(stack, stackEntry{name: item.Name, level: item.Level})
	}

	return relationships
}
