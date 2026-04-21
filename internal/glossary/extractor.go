package glossary

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
)

// rawEntry matches the JSON schema emitted by the LLM.
type rawEntry struct {
	Term       string   `json:"term"`
	Kind       string   `json:"kind"`
	Definition string   `json:"definition"`
	Aliases    []string `json:"aliases"`
}

// Extract parses the given HTML bytes, calls Claude to extract glossary entries,
// deduplicates on lowercase term, and returns the final slice.
func Extract(ctx context.Context, sourceFile string, htmlBytes []byte, codebase string, cfg *config.Config, client *claude.Client) ([]graph.GlossaryTerm, error) {
	text := ParseHTML(htmlBytes)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("no readable text found in %s", sourceFile)
	}

	tokenLimit := cfg.Glossary.TokenLimit
	if tokenLimit <= 0 {
		tokenLimit = 30000
	}
	maxTokens := cfg.Glossary.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 16000
	}

	// Rough char-to-token estimate (≈3.5 chars per token for English).
	const charsPerToken = 4
	chunkSize := tokenLimit * charsPerToken

	var allEntries []rawEntry

	if len(text) <= chunkSize {
		entries, err := callLLM(ctx, client, sourceFile, text, maxTokens)
		if err != nil {
			return nil, err
		}
		allEntries = append(allEntries, entries...)
	} else {
		// Split on blank lines to avoid cutting mid-entry.
		chunks := chunkText(text, chunkSize)
		for i, chunk := range chunks {
			label := fmt.Sprintf("%s (chunk %d/%d)", sourceFile, i+1, len(chunks))
			entries, err := callLLM(ctx, client, label, chunk, maxTokens)
			if err != nil {
				return nil, fmt.Errorf("chunk %d: %w", i+1, err)
			}
			allEntries = append(allEntries, entries...)
		}
	}

	return dedup(allEntries, codebase, sourceFile), nil
}

func callLLM(ctx context.Context, client *claude.Client, sourceFile, text string, maxTokens int) ([]rawEntry, error) {
	raw, err := client.ExtractGlossary(ctx, sourceFile, text, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction: %w", err)
	}

	// The LLM may return a JSON array or an object — normalize.
	raw = strings.TrimSpace(raw)
	// Strip stray trailing commas before closing bracket (model occasionally emits them)
	raw = strings.TrimRight(raw, " \t\n")

	var entries []rawEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("parsing LLM response as JSON array: %w (raw: %.200s)", err, raw)
	}
	return entries, nil
}

// chunkText splits text into chunks of at most maxChars, splitting on blank lines.
func chunkText(text string, maxChars int) []string {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	for _, para := range paragraphs {
		if current.Len()+len(para)+2 > maxChars && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// dedup deduplicates on lowercase term, returns the first occurrence of each.
func dedup(entries []rawEntry, codebase, sourceFile string) []graph.GlossaryTerm {
	seen := make(map[string]bool, len(entries))
	result := make([]graph.GlossaryTerm, 0, len(entries))

	cb := codebase
	if cb == "" {
		cb = "global"
	}

	for _, e := range entries {
		t := strings.TrimSpace(e.Term)
		if t == "" || e.Definition == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true

		kind := strings.ToLower(strings.TrimSpace(e.Kind))
		if kind != "acronym" && kind != "term" {
			kind = "term"
		}

		result = append(result, graph.GlossaryTerm{
			Term:       t,
			Kind:       kind,
			Definition: strings.TrimSpace(e.Definition),
			Aliases:    e.Aliases,
			Codebase:   cb,
			SourceFile: sourceFile,
		})
	}
	return result
}
