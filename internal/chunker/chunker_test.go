package chunker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cobol-ingestor/internal/graph"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBuildCopybookIndex(t *testing.T) {
	files := []graph.FileInfo{
		{Path: "/src/CUSTFILE.CPY", Type: graph.FileTypeCopybook},
		{Path: "/src/errhand.cpy", Type: graph.FileTypeCopybook},
		{Path: "/src/MAIN.CBL", Type: graph.FileTypeCOBOL},
	}

	idx := BuildCopybookIndex(files)

	assert.Equal(t, "/src/CUSTFILE.CPY", idx["CUSTFILE"])
	assert.Equal(t, "/src/errhand.cpy", idx["ERRHAND"])
	assert.Empty(t, idx["MAIN"]) // COBOL files excluded
	assert.Len(t, idx, 2)
}

func TestInlineCopybooks(t *testing.T) {
	dir := t.TempDir()

	// Write a copybook file
	cpyPath := filepath.Join(dir, "CUSTREC.CPY")
	require.NoError(t, os.WriteFile(cpyPath, []byte("       01  CUSTOMER-RECORD.\n           05  CUST-ID PIC X(10)."), 0644))

	index := CopybookIndex{"CUSTREC": cpyPath}

	content := "       IDENTIFICATION DIVISION.\n       DATA DIVISION.\n       COPY CUSTREC.\n       PROCEDURE DIVISION."
	result, err := InlineCopybooks(content, index, 10)
	require.NoError(t, err)

	assert.Contains(t, result, "*>> COPY CUSTREC INLINED BEGIN")
	assert.Contains(t, result, "CUSTOMER-RECORD")
	assert.Contains(t, result, "*>> COPY CUSTREC INLINED END")
	assert.NotContains(t, result, "COPY CUSTREC.")
}

func TestInlineCopybooks_Replacing(t *testing.T) {
	dir := t.TempDir()

	// Write a copybook with placeholder tags
	cpyPath := filepath.Join(dir, "CUSTREC.CPY")
	require.NoError(t, os.WriteFile(cpyPath, []byte("       01  :TAG:-RECORD.\n           05  :TAG:-ID PIC X(10)."), 0644))

	index := CopybookIndex{"CUSTREC": cpyPath}

	content := "       COPY CUSTREC REPLACING ==:TAG:== BY ==CUSTOMER==."
	result, err := InlineCopybooks(content, index, 10)
	require.NoError(t, err)

	assert.Contains(t, result, "CUSTOMER-RECORD")
	assert.Contains(t, result, "CUSTOMER-ID")
	assert.NotContains(t, result, ":TAG:")
}

func TestInlineCopybooks_MultipleReplacingPairs(t *testing.T) {
	dir := t.TempDir()

	cpyPath := filepath.Join(dir, "GENERIC.CPY")
	require.NoError(t, os.WriteFile(cpyPath, []byte("       01  :PFX:-:SFX:."), 0644))

	index := CopybookIndex{"GENERIC": cpyPath}

	content := "       COPY GENERIC REPLACING ==:PFX:== BY ==CUST== ==:SFX:== BY ==REC==."
	result, err := InlineCopybooks(content, index, 10)
	require.NoError(t, err)

	assert.Contains(t, result, "CUST-REC")
	assert.NotContains(t, result, ":PFX:")
	assert.NotContains(t, result, ":SFX:")
}

func TestSplitDivisions_FreeFormat(t *testing.T) {
	content := `IDENTIFICATION DIVISION.
PROGRAM-ID. TESTPROG.
DATA DIVISION.
WORKING-STORAGE SECTION.
01 WS-VAR PIC X(10).
PROCEDURE DIVISION.
0000-MAIN.
    DISPLAY "HELLO".
    STOP RUN.`

	divs := SplitDivisions(content)

	assert.Contains(t, divs, "IDENTIFICATION")
	assert.Contains(t, divs, "DATA")
	assert.Contains(t, divs, "PROCEDURE")
	assert.Contains(t, divs["PROCEDURE"], "0000-MAIN")
}

func TestInlineCopybooks_CircularReference(t *testing.T) {
	dir := t.TempDir()

	// Create two copybooks that reference each other
	cpyA := filepath.Join(dir, "A.CPY")
	cpyB := filepath.Join(dir, "B.CPY")
	require.NoError(t, os.WriteFile(cpyA, []byte("       COPY B."), 0644))
	require.NoError(t, os.WriteFile(cpyB, []byte("       COPY A."), 0644))

	index := CopybookIndex{"A": cpyA, "B": cpyB}

	// Should not hang or crash
	result, err := InlineCopybooks("       COPY A.", index, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestSplitDivisions(t *testing.T) {
	content := `       IDENTIFICATION DIVISION.
       PROGRAM-ID. TESTPROG.
       ENVIRONMENT DIVISION.
       INPUT-OUTPUT SECTION.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01 WS-VAR PIC X(10).
       PROCEDURE DIVISION.
       0000-MAIN.
           DISPLAY "HELLO".
           STOP RUN.`

	divs := SplitDivisions(content)

	assert.Contains(t, divs, "IDENTIFICATION")
	assert.Contains(t, divs, "ENVIRONMENT")
	assert.Contains(t, divs, "DATA")
	assert.Contains(t, divs, "PROCEDURE")
	assert.Contains(t, divs["IDENTIFICATION"], "PROGRAM-ID")
	assert.Contains(t, divs["PROCEDURE"], "0000-MAIN")
}

func TestSplitParagraphs(t *testing.T) {
	procedure := `       PROCEDURE DIVISION.
       0000-MAIN.
           PERFORM 1000-INIT.
           PERFORM 2000-PROCESS.
           STOP RUN.
       1000-INIT.
           DISPLAY "INIT".
       2000-PROCESS.
           DISPLAY "PROCESS".`

	paragraphs := splitParagraphs(procedure)

	// Should have preamble + 3 paragraphs
	names := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		names[i] = p.name
	}
	assert.Contains(t, names, "0000-MAIN")
	assert.Contains(t, names, "1000-INIT")
	assert.Contains(t, names, "2000-PROCESS")
}

func TestChunkFilePass2_SingleChunk(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "SMALL.CBL")

	content := `       IDENTIFICATION DIVISION.
       PROGRAM-ID. SMALL.
       PROCEDURE DIVISION.
       0000-MAIN.
           DISPLAY "HELLO".
           STOP RUN.`

	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	fi := graph.FileInfo{Path: filePath, Type: graph.FileTypeCOBOL}
	opts := Pass2ChunkOptions{TokenLimit: 100000, OverlapLines: 20}

	chunks, err := ChunkFilePass2(fi, opts, zap.NewNop())
	require.NoError(t, err)

	require.Len(t, chunks, 1)
	assert.Equal(t, 0, chunks[0].Index)
	assert.Equal(t, 1, chunks[0].Total)
	assert.Equal(t, 2, chunks[0].Pass)
}

func TestInlineCopybooks_UnderscoredName(t *testing.T) {
	dir := t.TempDir()

	cpyPath := filepath.Join(dir, "CUST_REC.CPY")
	require.NoError(t, os.WriteFile(cpyPath, []byte("       01  CUSTOMER-RECORD."), 0644))

	index := CopybookIndex{"CUST_REC": cpyPath}

	content := "       COPY CUST_REC."
	result, err := InlineCopybooks(content, index, 10)
	require.NoError(t, err)

	assert.Contains(t, result, "CUSTOMER-RECORD")
	assert.Contains(t, result, "*>> COPY CUST_REC INLINED BEGIN")
}

func TestSplitParagraphs_UnderscoredName(t *testing.T) {
	procedure := `       PROCEDURE DIVISION.
       PROCESS_CUSTOMER.
           DISPLAY "PROCESSING".
       VALIDATE_INPUT.
           DISPLAY "VALIDATING".`

	paragraphs := splitParagraphs(procedure)

	names := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		names[i] = p.name
	}
	assert.Contains(t, names, "PROCESS_CUSTOMER")
	assert.Contains(t, names, "VALIDATE_INPUT")
}

func TestNormalizeContinuations(t *testing.T) {
	// Column:  1234567890123456...
	// Line 1 has content, line 2 is a continuation (col 7 = '-')
	content := "      COPY CUST\n      -    REC."
	result := normalizeContinuations(content)
	assert.Contains(t, result, "CUSTREC.")
	assert.NotContains(t, result, "\n      -")
}

func TestChunkFile_MultiChunk(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "BIG.CBL")

	// Generate a file that will exceed a small token limit
	content := "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. BIG.\n       DATA DIVISION.\n       WORKING-STORAGE SECTION.\n       01 WS-VAR PIC X.\n"
	content += "       PROCEDURE DIVISION.\n"
	for i := 0; i < 20; i++ {
		content += fmt.Sprintf("       PARA-%04d.\n", i)
		for j := 0; j < 10; j++ {
			content += fmt.Sprintf("           DISPLAY \"LINE %d-%d\".\n", i, j)
		}
	}

	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	fi := graph.FileInfo{Path: filePath, Type: graph.FileTypeCOBOL}
	// Set a very low token limit to force splitting
	chunks, err := ChunkFile(fi, 500, zap.NewNop())
	require.NoError(t, err)

	assert.Greater(t, len(chunks), 1, "should produce multiple chunks for large file")
	for i, c := range chunks {
		assert.Equal(t, i, c.Index)
		assert.Equal(t, len(chunks), c.Total)
		assert.Equal(t, 1, c.Pass)
		if i == 0 {
			assert.Contains(t, c.Content, "IDENTIFICATION DIVISION") // full preamble in first chunk
		} else {
			assert.Contains(t, c.Content, "PREAMBLE SUMMARY") // summarized preamble in subsequent chunks
		}
	}
}

func TestChunkFile_SingleChunk(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "SMALL.CBL")

	content := "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. SMALL.\n       PROCEDURE DIVISION.\n       0000-MAIN.\n           DISPLAY \"HELLO\".\n           STOP RUN."

	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	fi := graph.FileInfo{Path: filePath, Type: graph.FileTypeCOBOL}
	chunks, err := ChunkFile(fi, 100000, zap.NewNop())
	require.NoError(t, err)

	require.Len(t, chunks, 1)
	assert.Equal(t, 0, chunks[0].Index)
	assert.Equal(t, 1, chunks[0].Total)
	assert.Equal(t, 1, chunks[0].Pass)
}

func TestSplitDataDivision(t *testing.T) {
	data := `       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-COUNTER PIC 9(4).
       01  WS-FLAG PIC X.
       FILE SECTION.
       FD  CUSTOMER-FILE.
       01  CUSTOMER-RECORD.
           05  CUST-ID PIC X(10).
       LINKAGE SECTION.
       01  LK-PARAM PIC X(100).`

	units := splitDataDivision(data)

	names := make([]string, len(units))
	for i, u := range units {
		names[i] = u.name
	}

	assert.Contains(t, names, "WORKING-STORAGE SECTION")
	assert.Contains(t, names, "FILE SECTION")
	assert.Contains(t, names, "LINKAGE SECTION")
}

func TestSummarizePreamble(t *testing.T) {
	divs := map[string]string{
		"IDENTIFICATION": "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. TESTPROG.",
		"ENVIRONMENT":    "       ENVIRONMENT DIVISION.\n       INPUT-OUTPUT SECTION.\n       FILE-CONTROL.\n           SELECT CUSTOMER-FILE ASSIGN TO 'CUST.DAT'.\n           SELECT REPORT-FILE ASSIGN TO 'RPT.DAT'.",
		"DATA": `       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-COUNTER PIC 9(4).
       01  WS-FLAG PIC X.
       FILE SECTION.
       FD  CUSTOMER-FILE.
       01  CUSTOMER-RECORD.
           05  CUST-ID PIC X(10).`,
	}

	summary := summarizePreamble(divs)

	assert.Contains(t, summary, "PROGRAM-ID. TESTPROG")
	assert.Contains(t, summary, "CUSTOMER-FILE")
	assert.Contains(t, summary, "REPORT-FILE")
	assert.Contains(t, summary, "WORKING-STORAGE SECTION")
	assert.Contains(t, summary, "WS-COUNTER")
	assert.Contains(t, summary, "CUSTOMER-RECORD")
	assert.Contains(t, summary, "PREAMBLE SUMMARY")

	// Summary should not contain full COBOL source lines
	assert.NotContains(t, summary, "PIC 9(4)")
	assert.NotContains(t, summary, "PIC X.")
}

func TestChunkFile_DataDivisionSplitting(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "BIGDATA.CBL")

	// Generate a file with a huge DATA DIVISION
	content := "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. BIGDATA.\n       DATA DIVISION.\n       WORKING-STORAGE SECTION.\n"
	for i := 0; i < 50; i++ {
		content += fmt.Sprintf("       01  WS-REC-%04d.\n", i)
		for j := 0; j < 10; j++ {
			content += fmt.Sprintf("           05  FIELD-%04d-%02d PIC X(100).\n", i, j)
		}
	}
	content += "       PROCEDURE DIVISION.\n       0000-MAIN.\n           DISPLAY 'DONE'.\n           STOP RUN.\n"

	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	fi := graph.FileInfo{Path: filePath, Type: graph.FileTypeCOBOL}
	// Use a limit that forces DATA to be split
	chunks, err := ChunkFile(fi, 2000, zap.NewNop())
	require.NoError(t, err)

	assert.Greater(t, len(chunks), 2, "should split DATA DIVISION into multiple chunks")
	// First chunk should have full preamble
	assert.Contains(t, chunks[0].Content, "IDENTIFICATION DIVISION")
	// Later chunks should have summary preamble
	if len(chunks) > 2 {
		assert.Contains(t, chunks[len(chunks)-1].Content, "PREAMBLE SUMMARY")
	}
}

func TestChunkFilePass2_MultiChunk(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "BIG.CBL")

	// Generate a file that will exceed a small token limit
	content := "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. BIG.\n       DATA DIVISION.\n       WORKING-STORAGE SECTION.\n       01 WS-VAR PIC X.\n"
	content += "       PROCEDURE DIVISION.\n"
	for i := 0; i < 20; i++ {
		content += fmt.Sprintf("       PARA-%04d.\n", i)
		for j := 0; j < 10; j++ {
			content += fmt.Sprintf("           DISPLAY \"LINE %d-%d\".\n", i, j)
		}
	}

	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	fi := graph.FileInfo{Path: filePath, Type: graph.FileTypeCOBOL}
	// Set a very low token limit to force splitting
	opts := Pass2ChunkOptions{TokenLimit: 500, OverlapLines: 5}

	chunks, err := ChunkFilePass2(fi, opts, zap.NewNop())
	require.NoError(t, err)

	assert.Greater(t, len(chunks), 1, "should produce multiple chunks")
	for i, c := range chunks {
		assert.Equal(t, i, c.Index)
		assert.Equal(t, len(chunks), c.Total)
		assert.Equal(t, 2, c.Pass)
		// All Pass 2 multi-chunks use summarized preamble
		assert.Contains(t, c.Content, "PREAMBLE SUMMARY")
	}
}

