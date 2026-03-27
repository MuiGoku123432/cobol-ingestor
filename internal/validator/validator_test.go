package validator

import (
	"testing"

	"cobol-ingestor/internal/graph"

	"github.com/stretchr/testify/assert"
)

func TestValidatePass1_Valid(t *testing.T) {
	result := &graph.Pass1Result{
		Programs: []graph.Program{{ProgramID: "TESTPROG", ExecutionMode: "BATCH"}},
		Paragraphs: []graph.Paragraph{
			{Name: "MAIN"}, {Name: "INIT"}, {Name: "PROCESS"},
		},
		DataItems: []graph.DataItem{{Name: "WS-REC"}},
	}
	v := New(nil)
	vr := v.ValidatePass1(result, 100)
	assert.True(t, vr.Valid)
	assert.InDelta(t, 1.0, vr.Score, 0.3)
}

func TestValidatePass1_NilResult(t *testing.T) {
	v := New(nil)
	vr := v.ValidatePass1(nil, 100)
	assert.False(t, vr.Valid)
	assert.Equal(t, 0.0, vr.Score)
}

func TestValidatePass1_UnknownProgramID(t *testing.T) {
	result := &graph.Pass1Result{
		Programs: []graph.Program{{ProgramID: "UNKNOWN", ExecutionMode: "UNKNOWN"}},
	}
	v := New(nil)
	vr := v.ValidatePass1(result, 0)
	assert.True(t, vr.Valid)
	assert.Less(t, vr.Score, 1.0)
	assert.NotEmpty(t, vr.Warnings)
}

func TestValidatePass2_GenericAnnotations(t *testing.T) {
	result := &graph.Pass2Result{
		Annotations: []graph.Annotation{
			{Paragraph: "MAIN", Description: "Performs main processing", Category: "PROCESSING"},
			{Paragraph: "INIT", Description: "Initializes customer record fields", Category: "INIT"},
		},
	}
	v := New(nil)
	vr := v.ValidatePass2(result, nil)
	assert.True(t, vr.Valid)
	assert.NotEmpty(t, vr.Warnings) // "main processing" is generic
}

func TestValidatePass3_LowCoverage(t *testing.T) {
	result := &graph.Pass3Result{
		DomainMembers: []graph.DomainMembership{
			{ProgramID: "P1", DomainName: "Domain1"},
		},
		RiskFlags: []graph.RiskFlag{
			{ProgramID: "P1", RiskType: "standard"},
		},
		VolumeEstimates: []graph.VolumeEstimate{
			{ProgramID: "P1", Estimate: "LOW"},
		},
	}
	v := New(nil)
	vr := v.ValidatePass3(result, 10) // 10 programs expected, only 1 covered
	assert.True(t, vr.Valid)
	assert.Less(t, vr.Score, 0.5)
	assert.NotEmpty(t, vr.Warnings)
}
