package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

// cPass1JSON mirrors the JSON schema returned by Claude for C structural analysis.
type cPass1JSON struct {
	Language         string              `json:"language"`
	Functions        []cFunctionJSON     `json:"functions"`
	Includes         []string            `json:"includes"`
	Structs          []cStructJSON       `json:"structs"`
	Typedefs         []cTypedefJSON      `json:"typedefs"`
	Globals          []cGlobalJSON       `json:"globals"`
	CallTargets      []CallTargetJSON    `json:"callTargets"`
	FileOperations   []cFileOpJSON       `json:"fileOperations"`
	OracleCalls      []cOracleCallJSON   `json:"oracleCalls"`
}

type cFunctionJSON struct {
	Name       string       `json:"name"`
	ReturnType string       `json:"returnType"`
	IsStatic   bool         `json:"isStatic"`
	Params     []cParamJSON `json:"params"`
}

type cParamJSON struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type cStructJSON struct {
	Name   string       `json:"name"`
	Fields []cParamJSON `json:"fields"`
}

type cTypedefJSON struct {
	Alias      string `json:"alias"`
	Underlying string `json:"underlying"`
}

type cGlobalJSON struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	IsExtern bool   `json:"isExtern"`
}

type cFileOpJSON struct {
	Operation       string `json:"operation"`
	FilePath        string `json:"filePath"`
	FunctionContext string `json:"functionContext"`
}

type cOracleCallJSON struct {
	FunctionName string `json:"functionName"`
	Context      string `json:"context"`
}

// CPass1Result holds C structural analysis output.
type CPass1Result struct {
	SourceFile    string
	Program       graph.CProgram
	Functions     []graph.CFunction
	Structs       []graph.CStruct
	Typedefs      []graph.CTypedef
	Headers       []graph.CHeader
	Relationships []graph.Relationship
}

// ParseCPass1Response parses Claude's JSON response for C structural analysis.
func ParseCPass1Response(jsonStr, sourceFile string) (*CPass1Result, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw cPass1JSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		recovered, recoverErr := RecoverPartialJSON(cleaned)
		if recoverErr != nil {
			return nil, fmt.Errorf("parsing C pass1 JSON: %w (recovery: %v)", err, recoverErr)
		}
		if err2 := json.Unmarshal([]byte(recovered), &raw); err2 != nil {
			return nil, fmt.Errorf("parsing recovered C pass1 JSON: %w", err2)
		}
	}

	result := &CPass1Result{
		SourceFile: sourceFile,
		Program: graph.CProgram{
			ID:       newID(),
			FilePath: sourceFile,
			Language: "C",
		},
	}

	// Functions
	for _, f := range raw.Functions {
		fn := graph.CFunction{
			ID:       newID(),
			Name:     f.Name,
			RetType:  f.ReturnType,
			IsStatic: f.IsStatic,
			FilePath: sourceFile,
			MergeID:  sourceFile + "::" + f.Name,
		}
		result.Functions = append(result.Functions, fn)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelDefines,
			FromLabel: "CProgram",
			FromKey:   sourceFile,
			ToLabel:   "CFunction",
			ToKey:     fn.MergeID,
		})
	}

	// Structs
	for _, s := range raw.Structs {
		st := graph.CStruct{
			ID:      newID(),
			Name:    s.Name,
			FilePath: sourceFile,
			MergeID: sourceFile + "::" + s.Name,
		}
		result.Structs = append(result.Structs, st)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelDefines,
			FromLabel: "CProgram",
			FromKey:   sourceFile,
			ToLabel:   "CStruct",
			ToKey:     st.MergeID,
		})
	}

	// Typedefs
	for _, t := range raw.Typedefs {
		td := graph.CTypedef{
			ID:         newID(),
			Alias:      t.Alias,
			Underlying: t.Underlying,
			FilePath:   sourceFile,
			MergeID:    sourceFile + "::" + t.Alias,
		}
		result.Typedefs = append(result.Typedefs, td)
	}

	// Includes → CHeader nodes + INCLUDES rels
	for _, inc := range raw.Includes {
		h := graph.CHeader{
			ID:      newID(),
			Name:    inc,
			MergeID: inc,
		}
		result.Headers = append(result.Headers, h)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelIncludes,
			FromLabel: "CProgram",
			FromKey:   sourceFile,
			ToLabel:   "CHeader",
			ToKey:     inc,
		})
	}

	// CALLS relationships
	for _, ct := range raw.CallTargets {
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelCalls,
			FromLabel: "CProgram",
			FromKey:   sourceFile,
			ToLabel:   "CFunction",
			ToKey:     ct.Target,
			Properties: map[string]any{"isDynamic": ct.IsDynamic},
		})
	}

	// File I/O → READS/WRITES rels
	for _, fo := range raw.FileOperations {
		relType := graph.RelReads
		switch fo.Operation {
		case "fwrite", "write", "fputs", "fprintf", "fputc":
			relType = graph.RelWrites
		}
		if fo.FilePath != "" {
			result.Relationships = append(result.Relationships, graph.Relationship{
				Type:      relType,
				FromLabel: "CProgram",
				FromKey:   sourceFile,
				ToLabel:   "ISAMFile",
				ToKey:     fo.FilePath,
				Properties: map[string]any{
					"operation": fo.Operation,
					"context":   fo.FunctionContext,
				},
			})
		}
	}

	return result, nil
}
