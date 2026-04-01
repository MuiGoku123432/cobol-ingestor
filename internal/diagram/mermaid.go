package diagram

import (
	"fmt"

	"github.com/yashikota/mermaigo/pkg/mermaid"
)

// RenderMermaidSVG renders Mermaid diagram code to an SVG string.
// theme can be any mermaigo theme name (e.g. "github-dark", "tokyo-night", "nord").
// Empty string defaults to "github-dark".
func RenderMermaidSVG(code string, theme string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("empty mermaid code")
	}

	if theme == "" {
		theme = "github-dark"
	}

	themeColors := mermaid.GetTheme(theme)
	opts := &mermaid.RenderOptions{
		Bg:      themeColors.Bg,
		Fg:      themeColors.Fg,
		Line:    themeColors.Line,
		Accent:  themeColors.Accent,
		Muted:   themeColors.Muted,
		Surface: themeColors.Surface,
		Border:  themeColors.Border,
	}

	svg, err := mermaid.Render(code, opts)
	if err != nil {
		return "", fmt.Errorf("mermaid render error: %w", err)
	}
	return svg, nil
}
