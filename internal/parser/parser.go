package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"cobol-ingestor/internal/graph"

	"github.com/google/uuid"
)

// Pass1JSON matches the JSON schema returned by Claude for Pass 1.
type Pass1JSON struct {
	ProgramID       string           `json:"programId"`
	CopyReferences  []string         `json:"copyReferences"`
	CallTargets     []CallTargetJSON `json:"callTargets"`
	Paragraphs      []string         `json:"paragraphs"`
	Sections        []string         `json:"sections"`
	FileDefinitions []FileDefJSON    `json:"fileDefinitions"`
	DataItems       []DataItemJSON   `json:"dataItems"`
	SQLStatements   []SQLJSON        `json:"sqlStatements"`
	CICSCommands    []CICSJSON       `json:"cicsCommands"`
}

type CallTargetJSON struct {
	Target    string `json:"target"`
	IsDynamic bool   `json:"isDynamic"`
}

type FileDefJSON struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
}

type DataItemJSON struct {
	Name    string `json:"name"`
	Level   int    `json:"level"`
	Picture string `json:"picture"`
}

type SQLJSON struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CICSJSON struct {
	Command string `json:"command"`
}

// ParsePass1Response parses Claude's JSON response into a Pass1Result.
func ParsePass1Response(jsonStr, sourceFile string) (*graph.Pass1Result, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw Pass1JSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing pass1 JSON: %w\nraw response: %.500s", err, cleaned)
	}

	result := &graph.Pass1Result{SourceFile: sourceFile}

	programID := raw.ProgramID
	if programID == "" {
		programID = "UNKNOWN"
	}

	// Program node
	prog := graph.Program{
		ID:        newID(),
		ProgramID: programID,
		FilePath:  sourceFile,
		Language:  "COBOL",
	}
	result.Programs = append(result.Programs, prog)

	// Copybooks + INCLUDES relationships
	for _, name := range raw.CopyReferences {
		cb := graph.Copybook{ID: newID(), Name: name}
		result.Copybooks = append(result.Copybooks, cb)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelIncludes,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "Copybook",
			ToKey:     name,
		})
	}

	// Call targets + CALLS relationships
	for _, ct := range raw.CallTargets {
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelCalls,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "Program",
			ToKey:     ct.Target,
			Properties: map[string]any{
				"isDynamic": ct.IsDynamic,
			},
		})
	}

	// Paragraphs + BELONGS_TO relationships
	for _, name := range raw.Paragraphs {
		para := graph.Paragraph{ID: newID(), Name: name, ProgramID: programID}
		result.Paragraphs = append(result.Paragraphs, para)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelBelongsTo,
			FromLabel: "Paragraph",
			FromKey:   name,
			ToLabel:   "Program",
			ToKey:     programID,
		})
	}

	// Sections + BELONGS_TO relationships
	for _, name := range raw.Sections {
		sec := graph.Section{ID: newID(), Name: name, ProgramID: programID}
		result.Sections = append(result.Sections, sec)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelBelongsTo,
			FromLabel: "Section",
			FromKey:   name,
			ToLabel:   "Program",
			ToKey:     programID,
		})
	}

	// File definitions
	for _, fd := range raw.FileDefinitions {
		fileDef := graph.FileDefinition{
			ID:           newID(),
			Name:         fd.Name,
			ProgramID:    programID,
			Organization: fd.Organization,
		}
		result.FileDefs = append(result.FileDefs, fileDef)
	}

	// Data items
	for _, di := range raw.DataItems {
		fqn := fmt.Sprintf("%s.%02d.%s", programID, di.Level, di.Name)
		item := graph.DataItem{
			ID:        newID(),
			Name:      di.Name,
			Level:     di.Level,
			ProgramID: programID,
			FQN:       fqn,
			Picture:   di.Picture,
		}
		result.DataItems = append(result.DataItems, item)
	}

	// SQL statements
	for _, sql := range raw.SQLStatements {
		stmt := graph.SQLStatement{
			ID:        newID(),
			Text:      sql.Text,
			ProgramID: programID,
			Type:      sql.Type,
		}
		result.SQLStatements = append(result.SQLStatements, stmt)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelExecutesSQL,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "SQLStatement",
			ToKey:     stmt.ID,
		})
	}

	// CICS transactions
	for _, cics := range raw.CICSCommands {
		txn := graph.CICSTransaction{
			ID:        newID(),
			Command:   cics.Command,
			ProgramID: programID,
		}
		result.CICSTxns = append(result.CICSTxns, txn)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelExecutesCICS,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "CICSTransaction",
			ToKey:     txn.ID,
		})
	}

	return result, nil
}

// stripMarkdownFences removes ```json ... ``` wrapping if present.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

func newID() string {
	return uuid.New().String()
}
