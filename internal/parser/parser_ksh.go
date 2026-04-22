package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

type kshPass1JSON struct {
	Shebang          string          `json:"shebang"`
	ScriptPurpose    string          `json:"scriptPurpose"`
	Functions        []kshFuncJSON   `json:"functions"`
	ExecutedBinaries []kshBinaryJSON `json:"executedBinaries"`
	SourcedFiles     []string        `json:"sourcedFiles"`
	EnvVars          []string        `json:"envVars"`
	FileOperations   []kshFileOpJSON `json:"fileOperations"`
	CallTargets      []kshCallJSON   `json:"callTargets"`
}

type kshFuncJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type kshBinaryJSON struct {
	Binary          string `json:"binary"`
	Args            string `json:"args"`
	FunctionContext string `json:"functionContext"`
}

type kshFileOpJSON struct {
	Path            string `json:"path"`
	Operation       string `json:"operation"`
	FunctionContext string `json:"functionContext"`
}

type kshCallJSON struct {
	Target          string `json:"target"`
	FunctionContext string `json:"functionContext"`
}

// KshPass1Result holds Korn shell structural analysis output.
type KshPass1Result struct {
	SourceFile    string
	Script        graph.ShellScript
	Functions     []graph.ShellFunction
	Relationships []graph.Relationship
}

// ParseKshPass1Response parses Claude's JSON response for ksh structural analysis.
func ParseKshPass1Response(jsonStr, sourceFile string) (*KshPass1Result, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw kshPass1JSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		recovered, recoverErr := RecoverPartialJSON(cleaned)
		if recoverErr != nil {
			return nil, fmt.Errorf("parsing ksh pass1 JSON: %w (recovery: %v)", err, recoverErr)
		}
		if err2 := json.Unmarshal([]byte(recovered), &raw); err2 != nil {
			return nil, fmt.Errorf("parsing recovered ksh pass1 JSON: %w", err2)
		}
	}

	result := &KshPass1Result{
		SourceFile: sourceFile,
		Script: graph.ShellScript{
			ID:       newID(),
			FilePath: sourceFile,
			Shebang:  raw.Shebang,
			Purpose:  raw.ScriptPurpose,
		},
	}

	// Functions
	for _, f := range raw.Functions {
		fn := graph.ShellFunction{
			ID:          newID(),
			Name:        f.Name,
			Description: f.Description,
			ScriptPath:  sourceFile,
			MergeID:     sourceFile + "::" + f.Name,
		}
		result.Functions = append(result.Functions, fn)
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelDefines,
			FromLabel: "ShellScript",
			FromKey:   sourceFile,
			ToLabel:   "ShellFunction",
			ToKey:     fn.MergeID,
		})
	}

	// Executed binaries → EXECUTES_BINARY rels
	for _, b := range raw.ExecutedBinaries {
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelExecutesBinary,
			FromLabel: "ShellScript",
			FromKey:   sourceFile,
			ToLabel:   "Program",
			ToKey:     b.Binary,
			Properties: map[string]any{
				"args":    b.Args,
				"context": b.FunctionContext,
			},
		})
	}

	// Sourced files → SOURCES rels
	for _, sf := range raw.SourcedFiles {
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelSources,
			FromLabel: "ShellScript",
			FromKey:   sourceFile,
			ToLabel:   "ShellScript",
			ToKey:     sf,
		})
	}

	// File operations → READS/WRITES rels
	for _, fo := range raw.FileOperations {
		relType := graph.RelReads
		if fo.Operation == "WRITE" || fo.Operation == "APPEND" {
			relType = graph.RelWrites
		}
		if fo.Path != "" {
			result.Relationships = append(result.Relationships, graph.Relationship{
				Type:      relType,
				FromLabel: "ShellScript",
				FromKey:   sourceFile,
				ToLabel:   "ISAMFile",
				ToKey:     fo.Path,
				Properties: map[string]any{"operation": fo.Operation},
			})
		}
	}

	// Internal function calls
	for _, ct := range raw.CallTargets {
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelCalls,
			FromLabel: "ShellScript",
			FromKey:   sourceFile,
			ToLabel:   "ShellFunction",
			ToKey:     sourceFile + "::" + ct.Target,
			Properties: map[string]any{"context": ct.FunctionContext},
		})
	}

	return result, nil
}
