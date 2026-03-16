package chunker

import (
	"strings"
	"testing"
)

func TestStripSequenceColumns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "standard 80-col line",
			// Cols 1-6: "000100", Cols 7-72: " IDENTIFICATION DIVISION.                                        M", Cols 73-80: "YID0001\x00"
			// line[6:72] keeps cols 7-72 (66 chars)
			input:    "000100 IDENTIFICATION DIVISION.                                        MYID0001",
			expected: " IDENTIFICATION DIVISION.                                        M",
		},
		{
			name:     "short line under 7 chars",
			input:    "SHORT",
			expected: "SHORT",
		},
		{
			name:     "exactly 7 chars",
			input:    "000100X",
			expected: "X",
		},
		{
			name:     "line with change markers in col 1-6",
			input:    "CHG001 MOVE A TO B.",
			expected: " MOVE A TO B.",
		},
		{
			name:     "line between 7 and 72 chars",
			input:    "000100       MOVE WS-FIELD TO WS-OTHER.",
			expected: "       MOVE WS-FIELD TO WS-OTHER.",
		},
		{
			name: "multiline stripping",
			input: "000100 IDENTIFICATION DIVISION.                                        MYID0001\n" +
				"000200 PROGRAM-ID. TESTPROG.                                            MYID0002\n" +
				"SHORT",
			expected: " IDENTIFICATION DIVISION.                                        M\n" +
				" PROGRAM-ID. TESTPROG.                                            \n" +
				"SHORT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSequenceColumns(tt.input)
			if got != tt.expected {
				t.Errorf("stripSequenceColumns()\ngot:  %q\nwant: %q", got, tt.expected)
			}
		})
	}
}

func TestCopyIDMS_SubschemaCtrl(t *testing.T) {
	input := "       COPY IDMS SUBSCHEMA-CTRL."
	matches := copyRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		t.Fatalf("expected match, got %v", matches)
	}
	if matches[1] != "SUBSCHEMA-CTRL" {
		t.Errorf("expected SUBSCHEMA-CTRL, got %s", matches[1])
	}
}

func TestCopyIDMS_Record(t *testing.T) {
	input := "       COPY IDMS RECORD MANUAL."
	matches := copyRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		t.Fatalf("expected match, got %v", matches)
	}
	if matches[1] != "MANUAL" {
		t.Errorf("expected MANUAL, got %s", matches[1])
	}
}

func TestCopyIDMS_Map(t *testing.T) {
	input := "       COPY IDMS MAP CUSTMAP."
	matches := copyRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		t.Fatalf("expected match, got %v", matches)
	}
	if matches[1] != "CUSTMAP" {
		t.Errorf("expected CUSTMAP, got %s", matches[1])
	}
}

func TestCopyIDMS_Module(t *testing.T) {
	input := "       COPY IDMS MODULE TESTMOD."
	matches := copyRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		t.Fatalf("expected match, got %v", matches)
	}
	if matches[1] != "TESTMOD" {
		t.Errorf("expected TESTMOD, got %s", matches[1])
	}
}

func TestCopyIDMS_Plain(t *testing.T) {
	// COPY IDMS without RECORD/MAP/MODULE qualifier
	input := "       COPY IDMS SUBSCHEMA-CTRL."
	matches := copyRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		t.Fatalf("expected match, got %v", matches)
	}
	if matches[1] != "SUBSCHEMA-CTRL" {
		t.Errorf("expected SUBSCHEMA-CTRL, got %s", matches[1])
	}
}

func TestCopyStandard_Unchanged(t *testing.T) {
	input := "       COPY CUSTFILE."
	matches := copyRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		t.Fatalf("expected match, got %v", matches)
	}
	if matches[1] != "CUSTFILE" {
		t.Errorf("expected CUSTFILE, got %s", matches[1])
	}
}

func TestCopyStandard_WithReplacing(t *testing.T) {
	input := "       COPY CUSTFILE REPLACING ==:TAG:== BY ==CUST==."
	matches := copyRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		t.Fatalf("expected match, got %v", matches)
	}
	if matches[1] != "CUSTFILE" {
		t.Errorf("expected CUSTFILE, got %s", matches[1])
	}
	if !strings.Contains(matches[2], "REPLACING") {
		t.Errorf("expected REPLACING clause in group 2, got %s", matches[2])
	}
}

func TestNormalizeContinuations_AfterStrip(t *testing.T) {
	// After stripping, indicator column is at index 0
	oldVal := StripSequenceColumns
	StripSequenceColumns = true
	defer func() { StripSequenceColumns = oldVal }()

	// Stripped content: column 7 is at index 0
	input := " MOVE 'VERY LONG STRING VALUE THAT\n" +
		"-    'CONTINUES HERE' TO WS-FIELD."

	result := normalizeContinuations(input)
	if strings.Contains(result, "\n-") {
		t.Errorf("continuation line not joined: %s", result)
	}
	if !strings.Contains(result, "CONTINUES HERE") {
		t.Errorf("continuation content missing: %s", result)
	}
}

func TestNormalizeContinuations_Unstripped(t *testing.T) {
	oldVal := StripSequenceColumns
	StripSequenceColumns = false
	defer func() { StripSequenceColumns = oldVal }()

	// Unstripped: indicator at index 6
	input := "000100       MOVE 'VERY LONG STRING VALUE THAT\n" +
		"000200-    'CONTINUES HERE' TO WS-FIELD."

	result := normalizeContinuations(input)
	if strings.Contains(result, "\n000200-") {
		t.Errorf("continuation line not joined: %s", result)
	}
	if !strings.Contains(result, "CONTINUES HERE") {
		t.Errorf("continuation content missing: %s", result)
	}
}

func TestSummarizePreamble_IDMS(t *testing.T) {
	divs := map[string]string{
		"IDENTIFICATION": "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. IDMSPROG.",
		"ENVIRONMENT":    "       ENVIRONMENT DIVISION.\n       IDMS-CONTROL SECTION.\n           PROTOCOL. MODE IS IDMS-DC.",
		"DATA":           "       DATA DIVISION.\n       SCHEMA SECTION.\n           DB EMPSS01 WITHIN EMPSCHM.",
	}

	summary := summarizePreamble(divs)

	if !strings.Contains(summary, "IDMSPROG") {
		t.Errorf("missing PROGRAM-ID in summary: %s", summary)
	}
	if !strings.Contains(summary, "IDMS-CONTROL: PROTOCOL MODE IDMS-DC") {
		t.Errorf("missing IDMS-CONTROL in summary: %s", summary)
	}
	if !strings.Contains(summary, "SCHEMA: subschema EMPSS01 within EMPSCHM") {
		t.Errorf("missing SCHEMA info in summary: %s", summary)
	}
}
