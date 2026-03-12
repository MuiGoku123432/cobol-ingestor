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
	ProgramID          string                  `json:"programId"`
	ExecutionMode      string                  `json:"executionMode"`
	CopyReferences     []string                `json:"copyReferences"`
	CallTargets        []CallTargetJSON        `json:"callTargets"`
	Paragraphs         []string                `json:"paragraphs"`
	Sections           []string                `json:"sections"`
	FileDefinitions    []FileDefJSON           `json:"fileDefinitions"`
	DataItems          []DataItemJSON          `json:"dataItems"`
	Conditions         []ConditionJSON         `json:"conditions"`
	Parameters         []ParameterJSON         `json:"parameters"`
	SQLStatements      []SQLJSON               `json:"sqlStatements"`
	CICSCommands       []CICSJSON              `json:"cicsCommands"`
	ExternalInterfaces []ExternalInterfaceJSON `json:"externalInterfaces"`
	DBTables           []DBTableJSON           `json:"dbTables"`
	IDMSSchemas        []IDMSSchemaJSON        `json:"idmsSchemas"`
	IDMSRecords        []IDMSRecordJSON        `json:"idmsRecords"`
	IDMSAreas          []IDMSAreaJSON          `json:"idmsAreas"`
	IDMSSets           []IDMSSetJSON           `json:"idmsSets"`
}

type DBTableJSON struct {
	Name       string   `json:"name"`
	Schema     string   `json:"schema"`
	Columns    []string `json:"columns"`
	Operations []string `json:"operations"`
}

type CallTargetJSON struct {
	Target    string `json:"target"`
	IsDynamic bool   `json:"isDynamic"`
}

type FileDefJSON struct {
	Name          string `json:"name"`
	Organization  string `json:"organization"`
	VSAMType      string `json:"vsamType"`
	DataStoreType string `json:"dataStoreType"`
}

type DataItemJSON struct {
	Name    string `json:"name"`
	Level   int    `json:"level"`
	Picture string `json:"picture"`
	Usage   string `json:"usage"`
}

type ConditionJSON struct {
	Name   string `json:"name"`
	Parent string `json:"parent"`
	Value  string `json:"value"`
}

type ParameterJSON struct {
	Name      string `json:"name"`
	Level     int    `json:"level"`
	Direction string `json:"direction"`
}

type SQLJSON struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	TargetTable string `json:"targetTable"`
}

type CICSJSON struct {
	Command string `json:"command"`
}

type ExternalInterfaceJSON struct {
	Type      string `json:"type"`
	Details   string `json:"details"`
	Paragraph string `json:"paragraph"`
}

type IDMSSchemaJSON struct {
	SchemaName    string `json:"schemaName"`
	SubschemaName string `json:"subschemaName"`
	ProtocolMode  string `json:"protocolMode"`
}

type IDMSRecordJSON struct {
	Name string `json:"name"`
	Area string `json:"area"`
}

type IDMSAreaJSON struct {
	Name      string `json:"name"`
	UsageMode string `json:"usageMode"`
}

type IDMSSetJSON struct {
	Name         string `json:"name"`
	OwnerRecord  string `json:"ownerRecord"`
	MemberRecord string `json:"memberRecord"`
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

	executionMode := raw.ExecutionMode
	if executionMode == "" {
		executionMode = "UNKNOWN"
	}

	// Program node
	prog := graph.Program{
		ID:            newID(),
		ProgramID:     programID,
		FilePath:      sourceFile,
		Language:      "COBOL",
		ExecutionMode: executionMode,
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
			ToKey:     strings.ToUpper(ct.Target),
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
			FromKey:   programID + "." + name,
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
			FromKey:   programID + "." + name,
			ToLabel:   "Program",
			ToKey:     programID,
		})
	}

	// File definitions
	for _, fd := range raw.FileDefinitions {
		fileDef := graph.FileDefinition{
			ID:            newID(),
			Name:          fd.Name,
			ProgramID:     programID,
			Organization:  fd.Organization,
			VSAMType:      fd.VSAMType,
			DataStoreType: fd.DataStoreType,
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
			Usage:     di.Usage,
		}
		result.DataItems = append(result.DataItems, item)
	}

	// Conditions (88-level)
	for _, c := range raw.Conditions {
		fqn := programID + "." + c.Name
		cond := graph.Condition{
			ID:        newID(),
			Name:      c.Name,
			Parent:    c.Parent,
			Value:     c.Value,
			ProgramID: programID,
			FQN:       fqn,
		}
		result.Conditions = append(result.Conditions, cond)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelConditionOf,
			FromLabel: "Condition",
			FromKey:   fqn,
			ToLabel:   "DataItem",
			ToKey:     c.Parent,
		})
	}

	// Parameters (LINKAGE SECTION items)
	for _, p := range raw.Parameters {
		fqn := programID + "." + p.Name
		param := graph.Parameter{
			ID:        newID(),
			Name:      p.Name,
			Level:     p.Level,
			Direction: p.Direction,
			ProgramID: programID,
			FQN:       fqn,
		}
		result.Parameters = append(result.Parameters, param)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelParameterOf,
			FromLabel: "Parameter",
			FromKey:   fqn,
			ToLabel:   "Program",
			ToKey:     programID,
		})
	}

	// SQL statements
	for _, sql := range raw.SQLStatements {
		stmt := graph.SQLStatement{
			ID:          newID(),
			Text:        sql.Text,
			ProgramID:   programID,
			Type:        sql.Type,
			TargetTable: sql.TargetTable,
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

	// DB Tables
	for _, dt := range raw.DBTables {
		table := graph.DBTable{
			ID:         newID(),
			Name:       dt.Name,
			Schema:     dt.Schema,
			Columns:    dt.Columns,
			Operations: dt.Operations,
		}
		result.DBTables = append(result.DBTables, table)
	}

	// IDMS Schemas
	for _, s := range raw.IDMSSchemas {
		schema := graph.IDMSSchema{
			ID:            newID(),
			SchemaName:    s.SchemaName,
			SubschemaName: s.SubschemaName,
			ProgramID:     programID,
			ProtocolMode:  s.ProtocolMode,
		}
		result.IDMSSchemas = append(result.IDMSSchemas, schema)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelBindsTo,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "IDMSSchema",
			ToKey:     schema.ID,
		})
	}

	// IDMS Records
	for _, r := range raw.IDMSRecords {
		rec := graph.IDMSRecord{
			ID:        newID(),
			Name:      r.Name,
			Area:      r.Area,
			ProgramID: programID,
		}
		result.IDMSRecords = append(result.IDMSRecords, rec)
	}

	// IDMS Areas
	for _, a := range raw.IDMSAreas {
		area := graph.IDMSArea{
			ID:        newID(),
			Name:      a.Name,
			UsageMode: a.UsageMode,
		}
		result.IDMSAreas = append(result.IDMSAreas, area)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelReadyArea,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "IDMSArea",
			ToKey:     a.Name,
			Properties: map[string]any{
				"usageMode": a.UsageMode,
			},
		})
	}

	// IDMS Sets
	for _, s := range raw.IDMSSets {
		set := graph.IDMSSet{
			ID:           newID(),
			Name:         s.Name,
			OwnerRecord:  s.OwnerRecord,
			MemberRecord: s.MemberRecord,
		}
		result.IDMSSets = append(result.IDMSSets, set)
	}

	// External interfaces
	for _, ei := range raw.ExternalInterfaces {
		iface := graph.ExternalInterface{
			ID:        newID(),
			Type:      ei.Type,
			Details:   ei.Details,
			Paragraph: ei.Paragraph,
			ProgramID: programID,
		}
		result.ExternalInterfaces = append(result.ExternalInterfaces, iface)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelExternalInterface,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "ExternalInterface",
			ToKey:     iface.ID,
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
	Performs               []PerformJSON              `json:"performs"`
	DataFlows              []DataFlowJSON             `json:"dataFlows"`
	FileOperations         []FileOpJSON               `json:"fileOperations"`
	SQLStatements          []SQLJSON                  `json:"sqlStatements"`
	CICSCommands           []CICSJSON                 `json:"cicsCommands"`
	DataHierarchy          []DataHierarchyJSON        `json:"dataHierarchy"`
	Redefines              []RedefineJSON             `json:"redefines"`
	CopybookDefinitions    []CopybookDefJSON          `json:"copybookDefinitions"`
	Annotations            []AnnotationJSON           `json:"annotations"`
	ConditionalLogic       []ConditionalLogicJSON     `json:"conditionalLogic"`
	DynamicCallResolution  []DynamicCallResolutionJSON `json:"dynamicCallResolution"`
	ErrorHandling          []ErrorHandlingJSON        `json:"errorHandling"`
	IDMSOperations         []IDMSOperationJSON        `json:"idmsOperations"`
}

type IDMSOperationJSON struct {
	Verb       string `json:"verb"`
	Record     string `json:"record"`
	Area       string `json:"area"`
	Set        string `json:"set"`
	CalcKey    string `json:"calcKey"`
	Navigation string `json:"navigation"`
	Paragraph  string `json:"paragraph"`
	UsageMode  string `json:"usageMode"`
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

type ConditionalLogicJSON struct {
	Paragraph string   `json:"paragraph"`
	Condition string   `json:"condition"`
	Variables []string `json:"variables"`
	Type      string   `json:"type"`
}

type DynamicCallResolutionJSON struct {
	Variable        string   `json:"variable"`
	ResolvedTargets []string `json:"resolvedTargets"`
	Paragraph       string   `json:"paragraph"`
}

type ErrorHandlingJSON struct {
	Paragraph string `json:"paragraph"`
	Pattern   string `json:"pattern"`
	Details   string `json:"details"`
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
			ID:          newID(),
			Text:        s.Text,
			ProgramID:   programID,
			Type:        s.Type,
			TargetTable: s.TargetTable,
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

	for _, cl := range raw.ConditionalLogic {
		result.ConditionalLogic = append(result.ConditionalLogic, graph.ConditionalLogicItem{
			Paragraph: cl.Paragraph,
			Condition: cl.Condition,
			Variables: cl.Variables,
			Type:      cl.Type,
		})
	}

	for _, dc := range raw.DynamicCallResolution {
		result.DynamicCallResolutions = append(result.DynamicCallResolutions, graph.DynamicCallResolution{
			Variable:        dc.Variable,
			ResolvedTargets: dc.ResolvedTargets,
			Paragraph:       dc.Paragraph,
		})
	}

	for _, eh := range raw.ErrorHandling {
		result.ErrorHandlers = append(result.ErrorHandlers, graph.ErrorHandler{
			Paragraph: eh.Paragraph,
			Pattern:   eh.Pattern,
			Details:   eh.Details,
		})
	}

	for _, op := range raw.IDMSOperations {
		result.IDMSOperations = append(result.IDMSOperations, graph.IDMSOperation{
			Verb:       op.Verb,
			Record:     op.Record,
			Area:       op.Area,
			Set:        op.Set,
			CalcKey:    op.CalcKey,
			Navigation: op.Navigation,
			Paragraph:  op.Paragraph,
			UsageMode:  op.UsageMode,
		})
	}

	return result, nil
}

func newID() string {
	return uuid.New().String()
}
