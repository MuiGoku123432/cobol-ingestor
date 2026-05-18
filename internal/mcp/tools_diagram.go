package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cobol-ingestor/internal/diagram"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerGenerateMermaidDiagram(s *mcp.Server, outputDir string) {
	type output struct {
		FilePath string `json:"filePath"`
		FileName string `json:"fileName"`
		Format   string `json:"format"`
		Success  bool   `json:"success"`
		SvgData  string `json:"svgData"`
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "generate_mermaid_diagram",
		Description: "Render a Mermaid diagram to SVG file. Use when the user asks for a visual diagram of call chains, data flows, program architecture, or any structural visualization.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GenerateMermaidDiagramInput) (*mcp.CallToolResult, any, error) {
		svg, err := diagram.RenderMermaidSVG(input.Code, input.Theme)
		if err != nil {
			return toolError(fmt.Sprintf("Mermaid render failed: %v", err)), nil, nil
		}

		fileName := input.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("diagram-%d", time.Now().Unix())
		}
		fileName += ".svg"

		filePath := filepath.Join(outputDir, fileName)
		if err := os.WriteFile(filePath, []byte(svg), 0644); err != nil {
			return toolError(fmt.Sprintf("Failed to write SVG: %v", err)), nil, nil
		}

		result := output{
			FilePath: filePath,
			FileName: fileName,
			Format:   "svg",
			Success:  true,
			SvgData:  svg,
		}

		b, _ := json.Marshal(result)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})
}
