package diagram

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractText_DrawIO_RealFile(t *testing.T) {
	// Use the real CARDDEMO-DataModel.drawio as a fixture
	content, err := os.ReadFile("../../aws-mainframe-modernization-carddemo/diagrams/CARDDEMO-DataModel.drawio")
	if err != nil {
		t.Skip("drawio fixture not available:", err)
	}

	result, err := ExtractText("CARDDEMO-DataModel.drawio", content)
	require.NoError(t, err)

	assert.Contains(t, result, "DIAGRAM FORMAT: DrawIO")
	assert.Contains(t, result, "PAGE:")
	// Should have at least one page with shapes
	assert.Contains(t, result, "SHAPES:")
	// The file has 2 pages: "ER Diagram" and "IMS"
	assert.Contains(t, result, "ER Diagram")
	assert.Contains(t, result, "IMS")
}

func TestExtractText_DrawIO_InlineXML(t *testing.T) {
	drawio := `<mxfile><diagram name="Test Page">
		<mxGraphModel>
			<root>
				<mxCell id="0"/>
				<mxCell id="1" parent="0"/>
				<mxCell id="2" value="Server" vertex="1" parent="1"/>
				<mxCell id="3" value="Database" vertex="1" parent="1"/>
				<mxCell id="4" value="connects to" edge="1" source="2" target="3" parent="1"/>
			</root>
		</mxGraphModel>
	</diagram></mxfile>`

	result, err := ExtractText("test.drawio", []byte(drawio))
	require.NoError(t, err)

	assert.Contains(t, result, "PAGE: Test Page")
	assert.Contains(t, result, "Server")
	assert.Contains(t, result, "Database")
	assert.Contains(t, result, "Server -> Database (connects to)")
}

func TestExtractText_PlantUML_Passthrough(t *testing.T) {
	puml := `@startuml
	Alice -> Bob: Authentication Request
	Bob --> Alice: Authentication Response
	@enduml`

	result, err := ExtractText("test.puml", []byte(puml))
	require.NoError(t, err)

	assert.Contains(t, result, "DIAGRAM FORMAT: PlantUML")
	assert.Contains(t, result, "SUBTYPE: UML")
	assert.Contains(t, result, "Alice -> Bob")
}

func TestExtractText_PlantUML_MindMap(t *testing.T) {
	puml := `@startmindmap
	* Root
	** Branch A
	** Branch B
	@endmindmap`

	result, err := ExtractText("test.plantuml", []byte(puml))
	require.NoError(t, err)

	assert.Contains(t, result, "SUBTYPE: Mind Map")
}

func TestExtractText_SVG(t *testing.T) {
	svg := `<?xml version="1.0"?>
	<svg xmlns="http://www.w3.org/2000/svg">
		<title>System Architecture</title>
		<desc>Overview of the system</desc>
		<text x="10" y="20">Web Server</text>
		<text x="10" y="40">Database</text>
		<g>
			<title>Connection Group</title>
			<text x="50" y="30">API Layer</text>
		</g>
	</svg>`

	result, err := ExtractText("test.svg", []byte(svg))
	require.NoError(t, err)

	assert.Contains(t, result, "DIAGRAM FORMAT: SVG")
	assert.Contains(t, result, "TITLE: System Architecture")
	assert.Contains(t, result, "DESCRIPTION: Overview of the system")
	assert.Contains(t, result, "Web Server")
	assert.Contains(t, result, "Database")
	assert.Contains(t, result, "API Layer")
}

func TestExtractText_VSDX_Minimal(t *testing.T) {
	// Create a minimal .vsdx (ZIP) with a page XML
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	pageXML := `<?xml version="1.0"?>
	<PageContents xmlns="http://schemas.microsoft.com/office/visio/2012/main">
		<Shapes>
			<Shape ID="1" NameU="Process">
				<Text>Order Processing</Text>
			</Shape>
			<Shape ID="2" NameU="Database">
				<Text>Customer DB</Text>
			</Shape>
		</Shapes>
	</PageContents>`

	f, err := w.Create("visio/pages/page1.xml")
	require.NoError(t, err)
	_, err = f.Write([]byte(pageXML))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := ExtractText("test.vsdx", buf.Bytes())
	require.NoError(t, err)

	assert.Contains(t, result, "DIAGRAM FORMAT: Visio")
	assert.Contains(t, result, "Order Processing")
	assert.Contains(t, result, "Customer DB")
}

func TestExtractText_UnsupportedFormat(t *testing.T) {
	_, err := ExtractText("test.png", []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported diagram format")
}

func TestExtractText_MalformedDrawIO(t *testing.T) {
	_, err := ExtractText("test.drawio", []byte("not xml at all"))
	assert.Error(t, err)
}

func TestIsDiagramExtension(t *testing.T) {
	assert.True(t, IsDiagramExtension(".drawio"))
	assert.True(t, IsDiagramExtension(".vsdx"))
	assert.True(t, IsDiagramExtension(".svg"))
	assert.True(t, IsDiagramExtension(".puml"))
	assert.True(t, IsDiagramExtension(".plantuml"))
	assert.True(t, IsDiagramExtension(".DRAWIO"))

	assert.False(t, IsDiagramExtension(".java"))
	assert.False(t, IsDiagramExtension(".xml"))
	assert.False(t, IsDiagramExtension(".png"))
}

func TestStripHTMLTags(t *testing.T) {
	assert.Equal(t, "Hello World", stripHTMLTags("<b>Hello</b> <i>World</i>"))
	assert.Equal(t, "Plain text", stripHTMLTags("Plain text"))
	assert.Equal(t, "", stripHTMLTags("<br/>"))
}

func TestDecompressDiagramContent_InlineXML(t *testing.T) {
	xml := `<mxGraphModel><root></root></mxGraphModel>`
	result, err := decompressDiagramContent(xml)
	require.NoError(t, err)
	assert.Equal(t, xml, string(result))
}

func TestDecompressDiagramContent_Empty(t *testing.T) {
	_, err := decompressDiagramContent("")
	assert.Error(t, err)
}

func TestDetectPlantUMLType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"@startuml\nclass Foo {}\n@enduml", "UML"},
		{"@startmindmap\n* root\n@endmindmap", "Mind Map"},
		{"@startgantt\n[Task] lasts 10 days\n@endgantt", "Gantt Chart"},
		{"plain text", ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, detectPlantUMLType(tt.input))
		})
	}
}

func TestExtractText_SVG_NoText(t *testing.T) {
	svg := `<?xml version="1.0"?>
	<svg xmlns="http://www.w3.org/2000/svg">
		<rect x="0" y="0" width="100" height="100"/>
	</svg>`

	result, err := ExtractText("empty.svg", []byte(svg))
	require.NoError(t, err)
	assert.Contains(t, result, "purely visual")
}

func TestExtractDrawIO_HTMLLabels(t *testing.T) {
	drawio := `<mxfile><diagram name="HTML Test">
		<mxGraphModel>
			<root>
				<mxCell id="0"/>
				<mxCell id="1" parent="0"/>
				<mxCell id="2" value="&lt;b&gt;Bold Label&lt;/b&gt;" vertex="1" parent="1"/>
			</root>
		</mxGraphModel>
	</diagram></mxfile>`

	result, err := ExtractText("test.drawio", []byte(drawio))
	require.NoError(t, err)
	assert.Contains(t, result, "Bold Label")
	// Shouldn't contain raw HTML tags
	assert.False(t, strings.Contains(result, "<b>"))
}
