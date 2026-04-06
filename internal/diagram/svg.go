package diagram

import (
	"encoding/xml"
	"fmt"
	"strings"
)

func extractSVG(content []byte) (string, error) {
	var out strings.Builder
	out.WriteString("DIAGRAM FORMAT: SVG\n\n")

	var texts []string
	var titles []string

	decoder := xml.NewDecoder(strings.NewReader(string(content)))
	var currentElement string
	var svgTitle, svgDesc string

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			currentElement = t.Name.Local
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text == "" {
				continue
			}
			switch currentElement {
			case "title":
				// First title at root level is the SVG title
				if svgTitle == "" && len(titles) == 0 {
					svgTitle = text
				}
				titles = append(titles, text)
			case "desc":
				if svgDesc == "" {
					svgDesc = text
				} else {
					titles = append(titles, text)
				}
			case "text", "tspan":
				texts = append(texts, text)
			}
		case xml.EndElement:
			currentElement = ""
		}
	}

	if svgTitle != "" {
		fmt.Fprintf(&out, "TITLE: %s\n", svgTitle)
	}
	if svgDesc != "" {
		fmt.Fprintf(&out, "DESCRIPTION: %s\n", svgDesc)
	}

	// Remove the SVG-level title from the titles list to avoid duplication
	if svgTitle != "" && len(titles) > 0 && titles[0] == svgTitle {
		titles = titles[1:]
	}

	if len(titles) > 0 {
		out.WriteString("\nELEMENT TITLES:\n")
		for _, t := range titles {
			fmt.Fprintf(&out, "  - %s\n", t)
		}
	}

	if len(texts) > 0 {
		out.WriteString("\nTEXT CONTENT:\n")
		for _, t := range texts {
			fmt.Fprintf(&out, "  - %s\n", t)
		}
	}

	if len(texts) == 0 && len(titles) == 0 && svgTitle == "" {
		out.WriteString("\n(No text content found in SVG — diagram may be purely visual)\n")
	}

	return out.String(), nil
}
