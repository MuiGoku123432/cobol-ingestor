package glossary

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// ParseHTML extracts readable text from an HTML glossary document.
// It preserves structural hints (definition-list boundaries, table-row boundaries,
// heading boundaries) as blank-line separators so Claude can infer term/definition pairs.
// Returns the normalized text string.
func ParseHTML(htmlBytes []byte) string {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		// Fall back to raw bytes as text — Claude can still handle messy HTML.
		return string(htmlBytes)
	}

	var buf strings.Builder
	extractText(doc, &buf)
	// Collapse runs of 3+ blank lines to 2.
	result := strings.ReplaceAll(buf.String(), "\n\n\n\n", "\n\n")
	result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	return strings.TrimSpace(result)
}

// extractText walks the HTML tree and appends text, inserting blank lines at
// structural boundaries that hint at glossary structure.
func extractText(n *html.Node, buf *strings.Builder) {
	if n.Type == html.TextNode {
		t := strings.TrimSpace(n.Data)
		if t != "" {
			buf.WriteString(t)
			buf.WriteRune('\n')
		}
		return
	}

	if n.Type == html.ElementNode {
		tag := strings.ToLower(n.Data)
		switch tag {
		case "script", "style", "head", "meta", "link", "noscript":
			return
		case "dt", "th":
			buf.WriteString("\n[TERM] ")
		case "dd", "td":
			buf.WriteString(" [DEF] ")
		case "tr":
			buf.WriteString("\n")
		case "h1", "h2", "h3", "h4", "h5", "h6":
			buf.WriteString("\n[HEADING] ")
		case "br", "p", "div", "li", "section", "article":
			buf.WriteString("\n")
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, buf)
	}

	if n.Type == html.ElementNode {
		tag := strings.ToLower(n.Data)
		switch tag {
		case "dt", "dd", "th", "td", "h1", "h2", "h3", "h4", "h5", "h6":
			buf.WriteString("\n")
		}
	}
}
