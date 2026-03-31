package scanner

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// createTestJAR creates a ZIP/JAR at the given path with the provided entries.
func createTestJAR(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)
	for name, data := range entries {
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
}

// createNestedJARBytes creates an in-memory JAR (ZIP) with the given entries and returns
// the raw bytes. Useful for embedding as a nested archive inside another JAR.
func createNestedJARBytes(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range entries {
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestExtractJAR_TextEntries(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")

	entries := map[string][]byte{
		"com/example/Foo.java":      []byte("public class Foo {}"),
		"META-INF/spring-beans.xml": []byte("<beans></beans>"),
		"config.properties":         []byte("key=value"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 3)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify content matches
	contentMap := make(map[string]string)
	for _, e := range result {
		contentMap[e.EntryPath] = string(e.Content)
	}
	assert.Equal(t, "public class Foo {}", contentMap["com/example/Foo.java"])
	assert.Equal(t, "<beans></beans>", contentMap["META-INF/spring-beans.xml"])
	assert.Equal(t, "key=value", contentMap["config.properties"])
}

func TestExtractJAR_SkipsNonAnalyzable(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")

	entries := map[string][]byte{
		"com/example/Main.java": []byte("class Main {}"),
		"images/logo.png":      []byte("fake-png-data"),
		"icons/app.gif":        []byte("fake-gif-data"),
		"static/icon.ico":      []byte("fake-ico-data"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 3)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "com/example/Main.java", result[0].EntryPath)
}

func TestExtractJAR_VirtualPaths(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "mylib.jar")

	entries := map[string][]byte{
		"com/example/Service.java": []byte("class Service {}"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 3)
	require.NoError(t, err)
	require.Len(t, result, 1)

	expected := jarPath + "!/com/example/Service.java"
	assert.Equal(t, expected, result[0].FileInfo.Path)
	assert.Contains(t, result[0].FileInfo.Path, "!/")
}

func TestExtractJAR_NestedJARExtracted(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "outer.jar")

	// Create a real nested JAR with analyzable content
	innerJARBytes := createNestedJARBytes(t, map[string][]byte{
		"com/inner/Helper.java": []byte("class Helper {}"),
		"inner-config.xml":      []byte("<config/>"),
	})

	entries := map[string][]byte{
		"lib/inner.jar":        innerJARBytes,
		"com/example/App.java": []byte("class App {}"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 3)
	require.NoError(t, err)

	// Should have: App.java + Helper.java + inner-config.xml = 3
	assert.Len(t, result, 3)

	// Verify virtual paths use chained !/
	pathMap := make(map[string]string)
	for _, e := range result {
		pathMap[e.FileInfo.Path] = string(e.Content)
	}

	// Top-level entry
	assert.Contains(t, pathMap, jarPath+"!/com/example/App.java")

	// Nested entries should have chained !/ paths
	assert.Contains(t, pathMap, jarPath+"!/lib/inner.jar!/com/inner/Helper.java")
	assert.Contains(t, pathMap, jarPath+"!/lib/inner.jar!/inner-config.xml")
}

func TestExtractJAR_NestedDepthLimit(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "deep.jar")

	// Create 4-level deep: deep.jar -> level1.jar -> level2.jar -> level3.jar -> Deep.java
	level3 := createNestedJARBytes(t, map[string][]byte{
		"com/Deep.java": []byte("class Deep {}"),
	})
	level2 := createNestedJARBytes(t, map[string][]byte{
		"lib/level3.jar": level3,
		"Level2.java":    []byte("class Level2 {}"),
	})
	level1 := createNestedJARBytes(t, map[string][]byte{
		"lib/level2.jar": level2,
		"Level1.java":    []byte("class Level1 {}"),
	})

	entries := map[string][]byte{
		"lib/level1.jar": level1,
		"Top.java":       []byte("class Top {}"),
	}
	createTestJAR(t, jarPath, entries)

	// maxDepth=2: depth 0 (top), depth 1 (level1), depth 2 (level2) — level3 should be skipped
	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 2)
	require.NoError(t, err)

	pathSet := make(map[string]bool)
	for _, e := range result {
		pathSet[e.FileInfo.Path] = true
	}

	// Top.java at depth 0
	assert.True(t, pathSet[jarPath+"!/Top.java"])
	// Level1.java at depth 1
	assert.True(t, pathSet[jarPath+"!/lib/level1.jar!/Level1.java"])
	// Level2.java at depth 2
	assert.True(t, pathSet[jarPath+"!/lib/level1.jar!/lib/level2.jar!/Level2.java"])
	// Deep.java at depth 3 — should NOT be present (level3.jar at depth 2 hits maxDepth)
	assert.False(t, pathSet[jarPath+"!/lib/level1.jar!/lib/level2.jar!/lib/level3.jar!/com/Deep.java"])
}

func TestExtractJAR_NestedWAR(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")

	// Create a WAR with WEB-INF entries
	warBytes := createNestedJARBytes(t, map[string][]byte{
		"WEB-INF/web.xml":                          []byte("<web-app/>"),
		"WEB-INF/classes/com/example/Servlet.java":  []byte("class Servlet {}"),
	})

	entries := map[string][]byte{
		"deploy/webapp.war":    warBytes,
		"com/example/App.java": []byte("class App {}"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 3)
	require.NoError(t, err)

	pathMap := make(map[string]bool)
	for _, e := range result {
		pathMap[e.FileInfo.Path] = true
	}

	assert.True(t, pathMap[jarPath+"!/com/example/App.java"])
	assert.True(t, pathMap[jarPath+"!/deploy/webapp.war!/WEB-INF/web.xml"])
	assert.True(t, pathMap[jarPath+"!/deploy/webapp.war!/WEB-INF/classes/com/example/Servlet.java"])
}

func TestExtractJAR_ManifestIncluded(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")

	entries := map[string][]byte{
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\nMain-Class: com.example.Main\n"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 3)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Contains(t, string(result[0].Content), "Main-Class")
}

func TestExtractJAR_HashConsistency(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")

	entries := map[string][]byte{
		"A.java": []byte("class A {}"),
		"B.java": []byte("class B {}"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger, 3)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// All entries from the same JAR should share the same hash
	assert.Equal(t, result[0].JARHash, result[1].JARHash)
	assert.Equal(t, result[0].FileInfo.Hash, result[1].FileInfo.Hash)
	assert.NotEmpty(t, result[0].JARHash)
}

func TestIsAnalyzableEntry(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"com/example/Foo.java", true},
		{"config.xml", true},
		{"app.properties", true},
		{"readme.txt", true},
		{"docs/guide.md", true},
		{"data.json", true},
		{"config.yml", true},
		{"config.yaml", true},
		{"settings.cfg", true},
		{"app.conf", true},
		{"com/example/Foo.class", true},
		{"META-INF/MANIFEST.MF", true},
		// Non-analyzable
		{"images/logo.png", false},
		{"images/photo.jpg", false},
		{"images/photo.jpeg", false},
		{"images/anim.gif", false},
		{"icons/app.ico", false},
		{"lib/native.so", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAnalyzableEntry(tt.name)
			assert.Equal(t, tt.expected, got, "isAnalyzableEntry(%q)", tt.name)
		})
	}
}

func TestIsNestedArchive(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"lib/inner.jar", true},
		{"app.war", true},
		{"app.ear", true},
		{"lib/deps.zip", true},
		{"LIB/INNER.JAR", true},
		{"com/example/Foo.java", false},
		{"config.xml", false},
		{"images/logo.png", false},
		{"lib/native.so", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNestedArchive(tt.name)
			assert.Equal(t, tt.expected, got, "isNestedArchive(%q)", tt.name)
		})
	}
}

func TestScanBW_WithJAR(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()

	// Create a regular Java file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Main.java"), []byte("class Main {}"), 0644))

	// Create a JAR with entries
	jarPath := filepath.Join(dir, "lib.jar")
	entries := map[string][]byte{
		"com/example/Service.java": []byte("class Service {}"),
		"config.xml":               []byte("<config/>"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ScanBW(context.Background(), dir, nil, logger, 3)
	require.NoError(t, err)

	// Should have: Main.java + 2 JAR entries = 3 files
	assert.Len(t, result.Files, 3)

	// Check JAR contents map
	var jarEntries int
	for _, f := range result.Files {
		if strings.Contains(f.Path, "!/") {
			jarEntries++
			content, ok := result.JARContents[f.Path]
			assert.True(t, ok, "JAR entry should have content in JARContents map: %s", f.Path)
			assert.NotEmpty(t, content)
		}
	}
	assert.Equal(t, 2, jarEntries)
}
