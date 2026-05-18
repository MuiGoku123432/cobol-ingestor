package diagram

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// Visio XML structures (simplified for shape/connector extraction)

type visioPage struct {
	XMLName xml.Name     `xml:"PageContents"`
	Shapes  []visioShape `xml:"Shapes>Shape"`
}

type visioShape struct {
	ID       string         `xml:"ID,attr"`
	NameU    string         `xml:"NameU,attr"`
	Name     string         `xml:"Name,attr"`
	Master   string         `xml:"Master,attr"`
	Type     string         `xml:"Type,attr"`
	Text     visioText      `xml:"Text"`
	Connects []visioConnect `xml:"Connects>Connect"`
	Shapes   []visioShape   `xml:"Shapes>Shape"`
}

type visioText struct {
	Content string `xml:",chardata"`
}

type visioConnects struct {
	XMLName  xml.Name       `xml:"Connects"`
	Connects []visioConnect `xml:"Connect"`
}

type visioConnect struct {
	FromSheet string `xml:"FromSheet,attr"`
	ToSheet   string `xml:"ToSheet,attr"`
	FromCell  string `xml:"FromCell,attr"`
}

type visioPages struct {
	XMLName xml.Name         `xml:"Pages"`
	Pages   []visioPageEntry `xml:"Page"`
}

type visioPageEntry struct {
	ID    string `xml:"ID,attr"`
	NameU string `xml:"NameU,attr"`
	Name  string `xml:"Name,attr"`
}

type visioMasters struct {
	XMLName xml.Name       `xml:"Masters"`
	Masters []visioMaster  `xml:"Master"`
}

type visioMaster struct {
	ID    string `xml:"ID,attr"`
	NameU string `xml:"NameU,attr"`
	Name  string `xml:"Name,attr"`
}

func extractVSDX(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("opening vsdx as ZIP: %w", err)
	}

	// Build master shape name lookup
	masterNames := loadMasterNames(reader)

	// Find page names from pages.xml
	pageNames := loadPageNames(reader)

	var out strings.Builder
	out.WriteString("DIAGRAM FORMAT: Visio (.vsdx)\n\n")

	pageNum := 0
	for _, f := range reader.File {
		if !isPageFile(f.Name) {
			continue
		}
		pageNum++

		pageName := fmt.Sprintf("Page %d", pageNum)
		if pageNum <= len(pageNames) {
			pageName = pageNames[pageNum-1]
		}
		fmt.Fprintf(&out, "PAGE: %s\n", pageName)

		rc, err := f.Open()
		if err != nil {
			fmt.Fprintf(&out, "  (error reading page: %v)\n\n", err)
			continue
		}
		pageData, err := readAllClose(rc)
		if err != nil {
			fmt.Fprintf(&out, "  (error reading page: %v)\n\n", err)
			continue
		}

		var page visioPage
		if err := xml.Unmarshal(pageData, &page); err != nil {
			fmt.Fprintf(&out, "  (error parsing page XML: %v)\n\n", err)
			continue
		}

		shapes, connectors := classifyVisioShapes(page.Shapes, masterNames)

		out.WriteString("SHAPES:\n")
		if len(shapes) == 0 {
			out.WriteString("  (none)\n")
		}
		for _, s := range shapes {
			masterInfo := ""
			if s.masterName != "" {
				masterInfo = fmt.Sprintf(" [%s]", s.masterName)
			}
			fmt.Fprintf(&out, "  - [%s] %s%s\n", s.id, s.label, masterInfo)
		}

		// Collect all connects from connectors and top-level
		allConnects := collectConnects(page.Shapes, connectors)

		out.WriteString("CONNECTIONS:\n")
		if len(allConnects) == 0 {
			out.WriteString("  (none)\n")
		}
		for _, c := range allConnects {
			srcLabel := findShapeLabel(shapes, c.from)
			tgtLabel := findShapeLabel(shapes, c.to)
			if srcLabel == "" {
				srcLabel = c.from
			}
			if tgtLabel == "" {
				tgtLabel = c.to
			}
			label := ""
			if c.label != "" {
				label = fmt.Sprintf(" (%s)", c.label)
			}
			fmt.Fprintf(&out, "  - %s -> %s%s\n", srcLabel, tgtLabel, label)
		}
		out.WriteString("\n")
	}

	if pageNum == 0 {
		return "", fmt.Errorf("no page files found in vsdx archive")
	}

	return out.String(), nil
}

type visioShapeInfo struct {
	id         string
	label      string
	masterName string
}

type visioConnInfo struct {
	from  string
	to    string
	label string
}

func classifyVisioShapes(shapes []visioShape, masterNames map[string]string) ([]visioShapeInfo, []visioShape) {
	var result []visioShapeInfo
	var connectors []visioShape

	for _, s := range shapes {
		text := strings.TrimSpace(s.Text.Content)
		name := s.NameU
		if name == "" {
			name = s.Name
		}

		// Connectors typically have Type="Group" with connects, or NameU starting with "Dynamic connector"
		isConnector := strings.Contains(strings.ToLower(name), "connector") ||
			strings.Contains(strings.ToLower(name), "dynamic") ||
			len(s.Connects) > 0

		if isConnector {
			connectors = append(connectors, s)
			continue
		}

		label := text
		if label == "" {
			label = name
		}
		if label == "" {
			label = fmt.Sprintf("Shape_%s", s.ID)
		}

		masterName := masterNames[s.Master]

		result = append(result, visioShapeInfo{
			id:         s.ID,
			label:      label,
			masterName: masterName,
		})

		// Recurse into sub-shapes
		subShapes, subConnectors := classifyVisioShapes(s.Shapes, masterNames)
		result = append(result, subShapes...)
		connectors = append(connectors, subConnectors...)
	}

	return result, connectors
}

func collectConnects(topShapes []visioShape, connectors []visioShape) []visioConnInfo {
	var result []visioConnInfo
	seen := make(map[string]bool)

	for _, conn := range connectors {
		for _, c := range conn.Connects {
			if c.FromCell == "BeginX" || c.FromCell == "" {
				// Look for the corresponding EndX connect
				for _, c2 := range conn.Connects {
					if c2.FromCell == "EndX" && c2.FromSheet == c.FromSheet {
						key := c.ToSheet + "->" + c2.ToSheet
						if !seen[key] {
							seen[key] = true
							result = append(result, visioConnInfo{
								from:  c.ToSheet,
								to:    c2.ToSheet,
								label: strings.TrimSpace(conn.Text.Content),
							})
						}
					}
				}
			}
		}
	}

	return result
}

func findShapeLabel(shapes []visioShapeInfo, id string) string {
	for _, s := range shapes {
		if s.id == id {
			return s.label
		}
	}
	return ""
}

func isPageFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "visio/pages/page") && strings.HasSuffix(lower, ".xml") &&
		!strings.HasSuffix(lower, "pages.xml")
}

func loadMasterNames(r *zip.Reader) map[string]string {
	names := make(map[string]string)
	for _, f := range r.File {
		if strings.ToLower(f.Name) == "visio/masters/masters.xml" {
			rc, err := f.Open()
			if err != nil {
				return names
			}
			data, err := readAllClose(rc)
			if err != nil {
				return names
			}
			var masters visioMasters
			if err := xml.Unmarshal(data, &masters); err != nil {
				return names
			}
			for _, m := range masters.Masters {
				name := m.NameU
				if name == "" {
					name = m.Name
				}
				names[m.ID] = name
			}
			return names
		}
	}
	return names
}

func loadPageNames(r *zip.Reader) []string {
	for _, f := range r.File {
		if strings.ToLower(f.Name) == "visio/pages/pages.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			data, err := readAllClose(rc)
			if err != nil {
				return nil
			}
			var pages visioPages
			if err := xml.Unmarshal(data, &pages); err != nil {
				return nil
			}
			var names []string
			for _, p := range pages.Pages {
				name := p.NameU
				if name == "" {
					name = p.Name
				}
				if name == "" {
					name = fmt.Sprintf("Page-%s", p.ID)
				}
				names = append(names, name)
			}
			return names
		}
	}
	return nil
}

func readAllClose(rc interface {
	Read([]byte) (int, error)
	Close() error
}) ([]byte, error) {
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
