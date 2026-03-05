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

// Pass2JSON matches the JSON schema returned by Claude for Pass 2.
type Pass2JSON struct {
	Performs            []PerformJSON       `json:"performs"`
	DataFlows           []DataFlowJSON      `json:"dataFlows"`
	FileOperations      []FileOpJSON        `json:"fileOperations"`
	SQLStatements       []SQLJSON           `json:"sqlStatements"`
	CICSCommands        []CICSJSON          `json:"cicsCommands"`
	DataHierarchy       []DataHierarchyJSON `json:"dataHierarchy"`
	Redefines           []RedefineJSON      `json:"redefines"`
	CopybookDefinitions []CopybookDefJSON   `json:"copybookDefinitions"`
	Annotations         []AnnotationJSON    `json:"annotations"`
}

type PerformJSON struct {
	FromParagraph string `json:"fromParagraph"`
	ToParagraph   string `json:"toParagraph"`
	ThruParagraph string `json:"thruParagraph"`
	IsLoop        bool   `json:"isLoop"`
	Condition     string `json:"condition"`
}

type DataFlowJSON struct {
	FromItem string `json:"fromItem"`
	ToItem   string `json:"toItem"`
	Context  string `json:"context"`
}

type FileOpJSON struct {
	Operation string `json:"operation"`
	FileName  string `json:"fileName"`
	Paragraph string `json:"paragraph"`
}

type DataHierarchyJSON struct {
	Name     string `json:"name"`
	Level    int    `json:"level"`
	Parent   string `json:"parent"`
	Picture  string `json:"picture"`
	Copybook string `json:"copybook"`
}

type RedefineJSON struct {
	Item      string `json:"item"`
	Redefines string `json:"redefines"`
}

type CopybookDefJSON struct {
	DataItem string `json:"dataItem"`
	Copybook string `json:"copybook"`
}

type AnnotationJSON struct {
	Paragraph   string `json:"paragraph"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// ParsePass2Response parses Claude's JSON response into a Pass2Result.
func ParsePass2Response(jsonStr, sourceFile, programID string) (*graph.Pass2Result, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw Pass2JSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing pass2 JSON: %w\nraw response: %.500s", err, cleaned)
	}

	result := &graph.Pass2Result{
		SourceFile: sourceFile,
		ProgramID:  programID,
	}

	for _, p := range raw.Performs {
		result.Performs = append(result.Performs, graph.PerformRelation{
			FromParagraph: p.FromParagraph,
			ToParagraph:   p.ToParagraph,
			ThruParagraph: p.ThruParagraph,
			IsLoop:        p.IsLoop,
			Condition:     p.Condition,
		})
	}

	for _, d := range raw.DataFlows {
		result.DataFlows = append(result.DataFlows, graph.DataFlowRelation{
			FromItem: d.FromItem,
			ToItem:   d.ToItem,
			Context:  d.Context,
		})
	}

	for _, f := range raw.FileOperations {
		result.FileOps = append(result.FileOps, graph.FileOpRelation{
			Operation: f.Operation,
			FileName:  f.FileName,
			Paragraph: f.Paragraph,
		})
	}

	for _, s := range raw.SQLStatements {
		result.SQLDetails = append(result.SQLDetails, graph.SQLStatement{
			ID:        newID(),
			Text:      s.Text,
			ProgramID: programID,
			Type:      s.Type,
		})
	}

	for _, c := range raw.CICSCommands {
		result.CICSDetails = append(result.CICSDetails, graph.CICSTransaction{
			ID:        newID(),
			Command:   c.Command,
			ProgramID: programID,
		})
	}

	for _, d := range raw.DataHierarchy {
		result.DataHierarchy = append(result.DataHierarchy, graph.DataHierarchyItem{
			Name:     d.Name,
			Level:    d.Level,
			Parent:   d.Parent,
			Picture:  d.Picture,
			Copybook: d.Copybook,
		})
	}

	for _, r := range raw.Redefines {
		result.Redefines = append(result.Redefines, graph.RedefineRelation{
			Item:      r.Item,
			Redefines: r.Redefines,
		})
	}

	for _, c := range raw.CopybookDefinitions {
		result.CopybookDefs = append(result.CopybookDefs, graph.CopybookDefRelation{
			DataItem: c.DataItem,
			Copybook: c.Copybook,
		})
	}

	for _, a := range raw.Annotations {
		result.Annotations = append(result.Annotations, graph.Annotation{
			Paragraph:   a.Paragraph,
			Description: a.Description,
			Category:    a.Category,
		})
	}

	return result, nil
}

func newID() string {
	return uuid.New().String()
}
