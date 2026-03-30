package diagram

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// drawio XML structures

type mxFile struct {
	XMLName  xml.Name        `xml:"mxfile"`
	Diagrams []mxFileDiagram `xml:"diagram"`
}

type mxFileDiagram struct {
	ID       string `xml:"id,attr"`
	Name     string `xml:"name,attr"`
	InnerXML string `xml:",innerxml"`
}

type mxGraphModel struct {
	XMLName xml.Name `xml:"mxGraphModel"`
	Root    mxRoot   `xml:"root"`
}

type mxRoot struct {
	Cells []mxCell `xml:"mxCell"`
}

type mxCell struct {
	ID     string `xml:"id,attr"`
	Value  string `xml:"value,attr"`
	Vertex string `xml:"vertex,attr"`
	Edge   string `xml:"edge,attr"`
	Source string `xml:"source,attr"`
	Target string `xml:"target,attr"`
	Parent string `xml:"parent,attr"`
	Style  string `xml:"style,attr"`
}

func extractDrawIO(content []byte) (string, error) {
	var file mxFile
	if err := xml.Unmarshal(content, &file); err != nil {
		return "", fmt.Errorf("parsing drawio XML: %w", err)
	}

	if len(file.Diagrams) == 0 {
		return "", fmt.Errorf("no <diagram> elements found in drawio file")
	}

	var out strings.Builder
	out.WriteString("DIAGRAM FORMAT: DrawIO\n\n")

	for i, diag := range file.Diagrams {
		name := diag.Name
		if name == "" {
			name = fmt.Sprintf("Page %d", i+1)
		}
		fmt.Fprintf(&out, "PAGE: %s\n", name)

		graphXML, err := decompressDiagramContent(strings.TrimSpace(diag.InnerXML))
		if err != nil {
			fmt.Fprintf(&out, "  (error decompressing page: %v)\n\n", err)
			continue
		}

		shapes, connections := parseMxGraphModel(graphXML)

		out.WriteString("SHAPES:\n")
		if len(shapes) == 0 {
			out.WriteString("  (none)\n")
		}
		for _, s := range shapes {
			fmt.Fprintf(&out, "  - [%s] %s\n", s.id, s.label)
		}

		out.WriteString("CONNECTIONS:\n")
		if len(connections) == 0 {
			out.WriteString("  (none)\n")
		}
		for _, c := range connections {
			label := ""
			if c.label != "" {
				label = fmt.Sprintf(" (%s)", c.label)
			}
			fmt.Fprintf(&out, "  - %s -> %s%s\n", c.sourceLabel, c.targetLabel, label)
		}
		out.WriteString("\n")
	}

	return out.String(), nil
}

// decompressDiagramContent handles compressed drawio diagram content:
// base64-decode -> deflate decompress -> URL-unescape.
// If content looks like inline XML, returns it directly.
func decompressDiagramContent(content string) ([]byte, error) {
	if content == "" {
		return nil, fmt.Errorf("empty diagram content")
	}

	// If it already looks like XML, return as-is
	if strings.HasPrefix(content, "<") {
		return []byte(content), nil
	}

	// base64 decode
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	// deflate decompress (raw deflate, not zlib)
	reader := flate.NewReader(bytes.NewReader(decoded))
	defer reader.Close()
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("deflate decompress: %w", err)
	}

	// URL unescape
	unescaped, err := url.QueryUnescape(string(decompressed))
	if err != nil {
		return nil, fmt.Errorf("URL unescape: %w", err)
	}

	return []byte(unescaped), nil
}

type shape struct {
	id    string
	label string
}

type connection struct {
	sourceID    string
	targetID    string
	sourceLabel string
	targetLabel string
	label       string
}

func parseMxGraphModel(xmlData []byte) ([]shape, []connection) {
	var model mxGraphModel
	if err := xml.Unmarshal(xmlData, &model); err != nil {
		return nil, nil
	}

	// Build ID->label map for resolving connection endpoints
	labelMap := make(map[string]string)
	var shapes []shape

	for _, cell := range model.Root.Cells {
		if cell.Vertex == "1" && cell.Value != "" {
			label := stripHTMLTags(cell.Value)
			if label != "" {
				labelMap[cell.ID] = label
				shapes = append(shapes, shape{id: cell.ID, label: label})
			}
		}
		// Also index cells with values that aren't explicitly vertices (group labels, etc.)
		if cell.Value != "" {
			if _, exists := labelMap[cell.ID]; !exists {
				labelMap[cell.ID] = stripHTMLTags(cell.Value)
			}
		}
	}

	var connections []connection
	for _, cell := range model.Root.Cells {
		if cell.Edge == "1" && cell.Source != "" && cell.Target != "" {
			srcLabel := labelMap[cell.Source]
			if srcLabel == "" {
				srcLabel = cell.Source
			}
			tgtLabel := labelMap[cell.Target]
			if tgtLabel == "" {
				tgtLabel = cell.Target
			}
			connections = append(connections, connection{
				sourceID:    cell.Source,
				targetID:    cell.Target,
				sourceLabel: srcLabel,
				targetLabel: tgtLabel,
				label:       stripHTMLTags(cell.Value),
			})
		}
	}

	return shapes, connections
}

// stripHTMLTags removes HTML tags from a string (drawio labels often contain HTML).
func stripHTMLTags(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
