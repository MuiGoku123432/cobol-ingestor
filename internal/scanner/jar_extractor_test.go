package scanner

import (
	"archive/zip"
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

	result, err := ExtractJAR(context.Background(), jarPath, "", logger)
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

	result, err := ExtractJAR(context.Background(), jarPath, "", logger)
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

	result, err := ExtractJAR(context.Background(), jarPath, "", logger)
	require.NoError(t, err)
	require.Len(t, result, 1)

	expected := jarPath + "!/com/example/Service.java"
	assert.Equal(t, expected, result[0].FileInfo.Path)
	assert.Contains(t, result[0].FileInfo.Path, "!/")
}

func TestExtractJAR_NestedJARSkipped(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "outer.jar")

	entries := map[string][]byte{
		"lib/inner.jar":        []byte("fake-jar-data"),
		"lib/webapp.war":       []byte("fake-war-data"),
		"com/example/App.java": []byte("class App {}"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger)
	require.NoError(t, err)
	// Only the .java file should be extracted
	assert.Len(t, result, 1)
	assert.Equal(t, "com/example/App.java", result[0].EntryPath)
}

func TestExtractJAR_ManifestIncluded(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")

	entries := map[string][]byte{
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\nMain-Class: com.example.Main\n"),
	}
	createTestJAR(t, jarPath, entries)

	result, err := ExtractJAR(context.Background(), jarPath, "", logger)
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

	result, err := ExtractJAR(context.Background(), jarPath, "", logger)
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
		{"lib/inner.jar", false},
		{"app.war", false},
		{"app.ear", false},
		{"lib/native.so", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAnalyzableEntry(tt.name)
			assert.Equal(t, tt.expected, got, "isAnalyzableEntry(%q)", tt.name)
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

	result, err := ScanBW(context.Background(), dir, nil, logger)
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
