package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergePass2Results_Empty(t *testing.T) {
	result := MergePass2Results(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result.Performs)
}

func TestMergePass2Results_Single(t *testing.T) {
	input := &Pass2Result{
		SourceFile: "test.cbl",
		ProgramID:  "TEST",
		Performs:    []PerformRelation{{FromParagraph: "A", ToParagraph: "B"}},
	}
	result := MergePass2Results([]*Pass2Result{input})
	assert.Equal(t, input, result)
}

func TestMergePass2Results_DeduplicatesPerforms(t *testing.T) {
	r1 := &Pass2Result{
		SourceFile: "test.cbl",
		ProgramID:  "TEST",
		Performs: []PerformRelation{
			{FromParagraph: "A", ToParagraph: "B"},
			{FromParagraph: "A", ToParagraph: "C"},
		},
	}
	r2 := &Pass2Result{
		SourceFile: "test.cbl",
		ProgramID:  "TEST",
		Performs: []PerformRelation{
			{FromParagraph: "A", ToParagraph: "B"}, // duplicate
			{FromParagraph: "C", ToParagraph: "D"},
		},
	}

	result := MergePass2Results([]*Pass2Result{r1, r2})
	assert.Len(t, result.Performs, 3) // A->B, A->C, C->D
}

func TestMergePass2Results_DeduplicatesDataFlows(t *testing.T) {
	r1 := &Pass2Result{
		SourceFile: "test.cbl",
		ProgramID:  "TEST",
		DataFlows: []DataFlowRelation{
			{FromItem: "X", ToItem: "Y", Context: "MAIN"},
		},
	}
	r2 := &Pass2Result{
		SourceFile: "test.cbl",
		ProgramID:  "TEST",
		DataFlows: []DataFlowRelation{
			{FromItem: "X", ToItem: "Y", Context: "MAIN"}, // duplicate
			{FromItem: "A", ToItem: "B", Context: "INIT"},
		},
	}

	result := MergePass2Results([]*Pass2Result{r1, r2})
	assert.Len(t, result.DataFlows, 2)
}

func TestMergePass2Results_AnnotationsKeepLongest(t *testing.T) {
	r1 := &Pass2Result{
		SourceFile: "test.cbl",
		ProgramID:  "TEST",
		Annotations: []Annotation{
			{Paragraph: "MAIN", Description: "short", Category: "PROCESSING"},
		},
	}
	r2 := &Pass2Result{
		SourceFile: "test.cbl",
		ProgramID:  "TEST",
		Annotations: []Annotation{
			{Paragraph: "MAIN", Description: "a much longer description of what MAIN does", Category: "PROCESSING"},
		},
	}

	result := MergePass2Results([]*Pass2Result{r1, r2})
	assert.Len(t, result.Annotations, 1)
	assert.Equal(t, "a much longer description of what MAIN does", result.Annotations[0].Description)
}
