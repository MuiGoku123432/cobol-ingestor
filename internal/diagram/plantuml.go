package diagram

import (
	"fmt"
	"strings"
)

func extractPlantUML(content []byte) (string, error) {
	text := string(content)

	var out strings.Builder
	out.WriteString("DIAGRAM FORMAT: PlantUML\n")

	// Detect diagram subtype from opening directive
	subtype := detectPlantUMLType(text)
	if subtype != "" {
		fmt.Fprintf(&out, "SUBTYPE: %s\n", subtype)
	}

	out.WriteString("\n")
	out.WriteString(text)

	return out.String(), nil
}

func detectPlantUMLType(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "@startuml"):
		return "UML"
	case strings.Contains(lower, "@startmindmap"):
		return "Mind Map"
	case strings.Contains(lower, "@startwbs"):
		return "Work Breakdown Structure"
	case strings.Contains(lower, "@startgantt"):
		return "Gantt Chart"
	case strings.Contains(lower, "@startjson"):
		return "JSON"
	case strings.Contains(lower, "@startyaml"):
		return "YAML"
	case strings.Contains(lower, "@startditaa"):
		return "Ditaa"
	case strings.Contains(lower, "@startsalt"):
		return "Salt (UI Wireframe)"
	default:
		return ""
	}
}
