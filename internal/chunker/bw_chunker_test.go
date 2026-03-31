package chunker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollapseBlankLines(t *testing.T) {
	input := "line1\n\n\n\nline2\n\n\n\n\nline3\n\nline4"
	got := collapseBlankLines(input)
	assert.Equal(t, "line1\n\nline2\n\nline3\n\nline4", got)
}

func TestCollapseBlankLines_TwoNewlinesUnchanged(t *testing.T) {
	input := "line1\n\nline2"
	got := collapseBlankLines(input)
	assert.Equal(t, input, got)
}

func TestTrimTrailingWhitespace(t *testing.T) {
	input := "hello   \nworld\t\t\nfoo  \t  \nbar"
	got := trimTrailingWhitespace(input)
	assert.Equal(t, "hello\nworld\nfoo\nbar", got)
}

func TestTrimTrailingWhitespace_PreservesLeading(t *testing.T) {
	input := "  indented  \n\ttabbed\t"
	got := trimTrailingWhitespace(input)
	assert.Equal(t, "  indented\n\ttabbed", got)
}

func TestShortenJava_BlockComments(t *testing.T) {
	input := `package com.example;

/**
 * This is a Javadoc comment.
 * It spans multiple lines.
 */
public class Foo {
    /* inline block comment */
    int x;
}`
	got := shortenJava(input)
	assert.NotContains(t, got, "Javadoc")
	assert.NotContains(t, got, "inline block comment")
	assert.Contains(t, got, "public class Foo")
	assert.Contains(t, got, "int x;")
}

func TestShortenJava_LineComments(t *testing.T) {
	input := `package com.example;

public class Foo {
    int x; // field comment
    // standalone comment
    void bar() {} // method comment
}`
	got := shortenJava(input)
	assert.NotContains(t, got, "field comment")
	assert.NotContains(t, got, "standalone comment")
	assert.NotContains(t, got, "method comment")
	assert.Contains(t, got, "int x;")
	assert.Contains(t, got, "void bar()")
}

func TestShortenJava_ImportSummary(t *testing.T) {
	var imports []string
	imports = append(imports,
		"import java.util.List;",
		"import java.util.Map;",
		"import java.util.ArrayList;",
		"import java.io.File;",
		"import java.io.IOException;",
		"import com.example.service.UserService;",
		"import com.example.service.OrderService;",
		"import com.example.model.User;",
		"import static org.junit.Assert.assertEquals;",
		"import static org.junit.Assert.assertTrue;",
	)

	input := "package com.example;\n\n" + strings.Join(imports, "\n") + "\n\npublic class Test {}"
	got := shortenJava(input)

	// Individual imports should be gone
	assert.NotContains(t, got, "import java.util.List;")
	assert.NotContains(t, got, "import com.example.service.UserService;")

	// Summary should be present
	assert.Contains(t, got, "// Imports:")
	assert.Contains(t, got, "java.util")
	assert.Contains(t, got, "java.io")
	assert.Contains(t, got, "com.example.service")
	assert.Contains(t, got, "com.example.model")
	assert.Contains(t, got, "// Static imports:")
	assert.Contains(t, got, "org.junit")
}

func TestShortenXML_Comments(t *testing.T) {
	input := `<?xml version="1.0"?>
<!-- This is a comment -->
<root>
  <!--
    Multi-line
    comment block
  -->
  <child>value</child>
</root>`
	got := shortenXML(input)
	assert.NotContains(t, got, "This is a comment")
	assert.NotContains(t, got, "Multi-line")
	assert.Contains(t, got, "<root>")
	assert.Contains(t, got, "<child>value</child>")
}

func TestSummarizeJavaPreamble(t *testing.T) {
	input := `package com.example.service;
// Imports: java.util, com.example.model
public class CustomerService extends BaseService {
    private String name;
    protected int count;
    public Logger log;
    public void processCustomer(Customer c) {
        // implementation
    }
    private int validate(String input) {
        return 0;
    }
}`
	got := summarizeJavaPreamble(input)

	assert.Contains(t, got, "PREAMBLE SUMMARY")
	assert.Contains(t, got, "package com.example.service;")
	assert.Contains(t, got, "// Imports: java.util, com.example.model")
	assert.Contains(t, got, "public class CustomerService extends BaseService {")
	assert.Contains(t, got, "// Fields:")
	assert.Contains(t, got, "// Methods:")
	assert.Contains(t, got, "void processCustomer()")
	assert.Contains(t, got, "int validate()")
}

func TestChunkBWFile_JavaShortening(t *testing.T) {
	// Build a Java file with lots of comments and imports that should be shortened
	var sb strings.Builder
	sb.WriteString("package com.example;\n\n")
	sb.WriteString("/**\n * Big Javadoc block.\n * Many lines.\n */\n")
	for i := 0; i < 20; i++ {
		sb.WriteString("import java.util.Something" + string(rune('A'+i)) + ";\n")
	}
	sb.WriteString("\npublic class BigClass {\n")
	// Generate enough methods to exceed a small token limit
	for i := 0; i < 50; i++ {
		sb.WriteString("    // method comment\n")
		sb.WriteString("    public void method" + strings.Repeat("X", 20) + string(rune('A'+i%26)) + "(String param) {\n")
		sb.WriteString("        " + strings.Repeat("doStuff(); ", 30) + "\n")
		sb.WriteString("    }\n\n")
	}
	sb.WriteString("}\n")

	input := sb.String()
	// Use a limit that forces multiple chunks but leaves room for overhead + preamble
	chunks, err := ChunkBWFile("Test.java", []byte(input), 5000)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should produce multiple chunks")

	// First chunk should NOT have preamble
	assert.NotContains(t, chunks[0].Content, "PREAMBLE SUMMARY")

	// Subsequent chunks should have preamble
	for i := 1; i < len(chunks); i++ {
		assert.Contains(t, chunks[i].Content, "PREAMBLE SUMMARY", "chunk %d should have preamble", i)
		assert.Contains(t, chunks[i].Content, "package com.example;")
	}

	// Comments should be stripped from all chunks
	for i, chunk := range chunks {
		assert.NotContains(t, chunk.Content, "Big Javadoc block", "chunk %d should not have Javadoc", i)
		assert.NotContains(t, chunk.Content, "method comment", "chunk %d should not have line comments", i)
	}

	// Imports should be summarized
	assert.Contains(t, chunks[0].Content, "// Imports:")
	assert.NotContains(t, chunks[0].Content, "import java.util.SomethingA;")
}

func TestChunkBWFile_PromptOverhead(t *testing.T) {
	// EstimateTokens: len(content) * 10 / 32 ≈ len/3.2
	// Build content with paragraphs totaling ~9600 chars ≈ 3000 tokens.
	// With tokenLimit=4000, effectiveLimit = 4000 - 2000 = 2000 → must chunk.
	// Without overhead, 3000 < 4000 → would be single chunk.
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString(strings.Repeat("x", 320)) // ~100 tokens per paragraph
		sb.WriteString("\n\n")
	}
	content := sb.String()

	chunks, err := ChunkBWFile("test.txt", []byte(content), 4000)
	require.NoError(t, err)
	assert.Greater(t, len(chunks), 1, "should be chunked due to prompt overhead reservation")
}

func TestChunkBWFile_SingleChunkUnchanged(t *testing.T) {
	input := `package com.example;

/**
 * Small class.
 */
import java.util.List;

public class Small {
    private int x;
    public void doSomething() {
        // logic here
    }
}`
	chunks, err := ChunkBWFile("Small.java", []byte(input), 50000)
	require.NoError(t, err)
	require.Len(t, chunks, 1)

	// Should have shortening applied
	assert.NotContains(t, chunks[0].Content, "Small class")
	assert.NotContains(t, chunks[0].Content, "logic here")
	assert.Contains(t, chunks[0].Content, "// Imports:")

	// No preamble for single chunk
	assert.NotContains(t, chunks[0].Content, "PREAMBLE SUMMARY")

	// Trailing whitespace should be trimmed (the "   " after "int x")
	assert.NotContains(t, chunks[0].Content, "int x;   ")

	assert.Equal(t, 0, chunks[0].Index)
	assert.Equal(t, 1, chunks[0].Total)
}
