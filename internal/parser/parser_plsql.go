package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

type plsqlPass1JSON struct {
	ObjectType        string               `json:"objectType"`
	ObjectName        string               `json:"objectName"`
	SchemaName        string               `json:"schemaName"`
	Procedures        []plsqlProcJSON      `json:"procedures"`
	Functions         []plsqlFuncJSON      `json:"functions"`
	Cursors           []plsqlCursorJSON    `json:"cursors"`
	Triggers          []plsqlTriggerJSON   `json:"triggers"`
	CallTargets       []plsqlCallJSON      `json:"callTargets"`
	TableReferences   []plsqlTableRefJSON  `json:"tableReferences"`
	ExceptionHandlers []plsqlExcJSON       `json:"exceptionHandlers"`
	UTLFileRefs       []plsqlUTLFileJSON   `json:"utlFileRefs"`
}

type plsqlProcJSON struct {
	Name   string          `json:"name"`
	Params []plsqlParamJSON `json:"params"`
}

type plsqlFuncJSON struct {
	Name       string          `json:"name"`
	ReturnType string          `json:"returnType"`
	Params     []plsqlParamJSON `json:"params"`
}

type plsqlParamJSON struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Direction string `json:"direction"`
}

type plsqlCursorJSON struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

type plsqlTriggerJSON struct {
	Name      string `json:"name"`
	TableName string `json:"tableName"`
	Event     string `json:"event"`
	Timing    string `json:"timing"`
}

type plsqlCallJSON struct {
	PackageName string `json:"packageName"`
	ProcName    string `json:"procName"`
}

type plsqlTableRefJSON struct {
	TableName  string   `json:"tableName"`
	SchemaName string   `json:"schemaName"`
	Operations []string `json:"operations"`
}

type plsqlExcJSON struct {
	ExceptionName string `json:"exceptionName"`
	Context       string `json:"context"`
}

type plsqlUTLFileJSON struct {
	Operation string `json:"operation"`
	FilePath  string `json:"filePath"`
	Context   string `json:"context"`
}

// PLSQLPass1Result holds PL/SQL structural analysis output.
type PLSQLPass1Result struct {
	SourceFile    string
	Package       graph.PLSQLPackage
	Procedures    []graph.PLSQLProcedure
	Functions     []graph.PLSQLFunction
	Triggers      []graph.PLSQLTrigger
	Cursors       []graph.PLSQLCursor
	Relationships []graph.Relationship
}

// ParsePLSQLPass1Response parses Claude's JSON response for PL/SQL structural analysis.
func ParsePLSQLPass1Response(jsonStr, sourceFile string) (*PLSQLPass1Result, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw plsqlPass1JSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		recovered, recoverErr := RecoverPartialJSON(cleaned)
		if recoverErr != nil {
			return nil, fmt.Errorf("parsing PL/SQL pass1 JSON: %w (recovery: %v)", err, recoverErr)
		}
		if err2 := json.Unmarshal([]byte(recovered), &raw); err2 != nil {
			return nil, fmt.Errorf("parsing recovered PL/SQL pass1 JSON: %w", err2)
		}
	}

	result := &PLSQLPass1Result{
		SourceFile: sourceFile,
		Package: graph.PLSQLPackage{
			ID:         newID(),
			Name:       raw.ObjectName,
			ObjectType: raw.ObjectType,
			SchemaName: raw.SchemaName,
			FilePath:   sourceFile,
		},
	}

	// Procedures
	for _, p := range raw.Procedures {
		proc := graph.PLSQLProcedure{
			ID:          newID(),
			Name:        p.Name,
			PackageName: raw.ObjectName,
			FilePath:    sourceFile,
			MergeID:     raw.ObjectName + "." + p.Name,
		}
		result.Procedures = append(result.Procedures, proc)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelDefines,
			FromLabel: "PLSQLPackage",
			FromKey:   raw.ObjectName,
			ToLabel:   "PLSQLProcedure",
			ToKey:     proc.MergeID,
		})
	}

	// Functions
	for _, f := range raw.Functions {
		fn := graph.PLSQLFunction{
			ID:          newID(),
			Name:        f.Name,
			ReturnType:  f.ReturnType,
			PackageName: raw.ObjectName,
			FilePath:    sourceFile,
			MergeID:     raw.ObjectName + "." + f.Name,
		}
		result.Functions = append(result.Functions, fn)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelDefines,
			FromLabel: "PLSQLPackage",
			FromKey:   raw.ObjectName,
			ToLabel:   "PLSQLFunction",
			ToKey:     fn.MergeID,
		})
	}

	// Triggers
	for _, t := range raw.Triggers {
		trig := graph.PLSQLTrigger{
			ID:        newID(),
			Name:      t.Name,
			TableName: t.TableName,
			Event:     t.Event,
			Timing:    t.Timing,
			FilePath:  sourceFile,
			MergeID:   sourceFile + "::" + t.Name,
		}
		result.Triggers = append(result.Triggers, trig)
		if t.TableName != "" {
			result.Relationships = append(result.Relationships, graph.Relationship{
				Type:      graph.RelFiresOn,
				FromLabel: "PLSQLTrigger",
				FromKey:   trig.MergeID,
				ToLabel:   "ExternalDBTable",
				ToKey:     t.TableName,
				Properties: map[string]any{"event": t.Event, "timing": t.Timing},
			})
		}
	}

	// Cursors
	for _, c := range raw.Cursors {
		cur := graph.PLSQLCursor{
			ID:          newID(),
			Name:        c.Name,
			Query:       c.Query,
			PackageName: raw.ObjectName,
			MergeID:     raw.ObjectName + ".cursor." + c.Name,
		}
		result.Cursors = append(result.Cursors, cur)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelOwnsCursor,
			FromLabel: "PLSQLPackage",
			FromKey:   raw.ObjectName,
			ToLabel:   "PLSQLCursor",
			ToKey:     cur.MergeID,
		})
	}

	// Table references → USES_TABLE rels on shared ExternalDBTable nodes
	for _, tr := range raw.TableReferences {
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelUsesOracleObj,
			FromLabel: "PLSQLPackage",
			FromKey:   raw.ObjectName,
			ToLabel:   "ExternalDBTable",
			ToKey:     tr.TableName,
			Properties: map[string]any{"operations": tr.Operations},
		})
	}

	// UTL_FILE refs → READS/WRITES rels on ISAMFile nodes
	for _, u := range raw.UTLFileRefs {
		relType := graph.RelReads
		switch u.Operation {
		case "PUT_LINE", "PUT", "FFLUSH":
			relType = graph.RelWrites
		}
		if u.FilePath != "" {
			result.Relationships = append(result.Relationships, graph.Relationship{
				Type:      relType,
				FromLabel: "PLSQLPackage",
				FromKey:   raw.ObjectName,
				ToLabel:   "ISAMFile",
				ToKey:     u.FilePath,
				Properties: map[string]any{"operation": u.Operation, "context": u.Context},
			})
		}
	}

	// CALLS relationships
	for _, ct := range raw.CallTargets {
		target := ct.ProcName
		if ct.PackageName != "" {
			target = ct.PackageName + "." + ct.ProcName
		}
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelCalls,
			FromLabel: "PLSQLPackage",
			FromKey:   raw.ObjectName,
			ToLabel:   "PLSQLProcedure",
			ToKey:     target,
		})
	}

	return result, nil
}
