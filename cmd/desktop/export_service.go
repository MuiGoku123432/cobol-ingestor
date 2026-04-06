package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportService generates downloadable reports from the graph data.
type ExportService struct {
	app *App
}

func (s *ExportService) reader() (n4j.Reader, error) {
	r := s.app.Neo4jService.reader
	if r == nil {
		return nil, fmt.Errorf("not connected to Neo4j")
	}
	return r, nil
}

// SelectSaveDirectory opens a native OS directory picker.
func (s *ExportService) SelectSaveDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(s.app.ctx, runtime.OpenDialogOptions{
		Title: "Select Export Directory",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// ExportAnalysisReport generates a comprehensive markdown report.
func (s *ExportService) ExportAnalysisReport(outputDir string) (string, error) {
	r, err := s.reader()
	if err != nil {
		return "", err
	}
	ctx := context.Background()

	var sb strings.Builder
	sb.WriteString("# COBOL Codebase Analysis Report\n\n")

	// Dashboard stats
	if stats, err := r.GetDashboardStats(ctx); err == nil {
		sb.WriteString("## Dashboard Overview\n\n")
		sb.WriteString(fmt.Sprintf("| Metric | Count |\n|--------|-------|\n"))
		sb.WriteString(fmt.Sprintf("| Programs | %d |\n", stats.ProgramCount))
		sb.WriteString(fmt.Sprintf("| Copybooks | %d |\n", stats.CopybookCount))
		sb.WriteString(fmt.Sprintf("| Paragraphs | %d |\n", stats.ParagraphCount))
		sb.WriteString(fmt.Sprintf("| Data Items | %d |\n", stats.DataItemCount))
		sb.WriteString(fmt.Sprintf("| SQL Statements | %d |\n", stats.SQLStatementCount))
		sb.WriteString(fmt.Sprintf("| CICS Transactions | %d |\n", stats.CICSTransactionCount))
		sb.WriteString(fmt.Sprintf("| Relationships | %d |\n", stats.RelationshipCount))
		sb.WriteString(fmt.Sprintf("| Business Domains | %d |\n", stats.DomainCount))
		sb.WriteString(fmt.Sprintf("| Orphan Programs | %d |\n", stats.OrphanCount))
		sb.WriteString(fmt.Sprintf("| DB Tables | %d |\n\n", stats.DBTableCount))
	}

	// Modernization candidates
	if candidates, err := r.ListModernizationCandidates(ctx); err == nil && len(candidates) > 0 {
		sb.WriteString("## Modernization Candidates\n\n")
		sb.WriteString("| Program | Score | Approach | Reason |\n|---------|-------|----------|--------|\n")
		for _, c := range candidates {
			sb.WriteString(fmt.Sprintf("| %s | %.1f | %s | %s |\n", c.ProgramID, c.Score, c.Approach, c.Reason))
		}
		sb.WriteString("\n")
	}

	// Risk programs
	if risks, err := r.ListRiskPrograms(ctx, 0); err == nil && len(risks) > 0 {
		sb.WriteString("## Risk Programs\n\n")
		sb.WriteString("| Program | Risk Score | Risk Type | Details |\n|---------|------------|-----------|--------|\n")
		for _, rp := range risks {
			sb.WriteString(fmt.Sprintf("| %s | %.1f | %s | %s |\n", rp.ProgramID, rp.RiskScore, rp.RiskType, rp.RiskDetails))
		}
		sb.WriteString("\n")
	}

	// Migration sequence
	if steps, err := r.GetMigrationSequence(ctx); err == nil && len(steps) > 0 {
		sb.WriteString("## Migration Sequence\n\n")
		sb.WriteString("| Order | Program | Tier | Score | Blocked By |\n|-------|---------|------|-------|------------|\n")
		for _, step := range steps {
			blocked := strings.Join(step.BlockedBy, ", ")
			if blocked == "" {
				blocked = "-"
			}
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %.1f | %s |\n", step.Order, step.ProgramID, step.Tier, step.Score, blocked))
		}
		sb.WriteString("\n")
	}

	// Effort estimates
	if estimates, err := r.GetEffortEstimates(ctx); err == nil && len(estimates) > 0 {
		sb.WriteString("## Effort Estimates\n\n")
		sb.WriteString("| Program | T-Shirt | Complexity | Lines | Paragraphs | Copybooks | SQL | CICS |\n")
		sb.WriteString("|---------|---------|------------|-------|------------|-----------|-----|------|\n")
		for _, e := range estimates {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %d | %d | %d |\n",
				e.ProgramID, e.TShirtSize, e.ComplexityScore, e.LineCount,
				e.ParagraphCount, e.CopybookCount, e.SQLCount, e.CICSCount))
		}
		sb.WriteString("\n")
	}

	outPath := filepath.Join(outputDir, "analysis-report.md")
	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("writing report: %w", err)
	}
	return outPath, nil
}

// ExportProgramDetail generates a markdown report for a single program.
func (s *ExportService) ExportProgramDetail(programID, outputDir string) (string, error) {
	r, err := s.reader()
	if err != nil {
		return "", err
	}
	ctx := context.Background()

	prog, err := r.GetProgram(ctx, programID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Program: %s\n\n", prog.ProgramID))
	sb.WriteString(fmt.Sprintf("- **File:** %s\n", prog.FilePath))
	sb.WriteString(fmt.Sprintf("- **Language:** %s\n", prog.Language))
	sb.WriteString(fmt.Sprintf("- **Lines:** %d\n", prog.LineCount))
	sb.WriteString(fmt.Sprintf("- **Execution Mode:** %s\n", prog.ExecutionMode))
	sb.WriteString(fmt.Sprintf("- **Dead Code:** %v\n", prog.DeadCode))
	if prog.RiskScore > 0 {
		sb.WriteString(fmt.Sprintf("- **Risk Score:** %.1f (%s)\n", prog.RiskScore, prog.RiskType))
	}
	sb.WriteString("\n")

	if len(prog.Callers) > 0 {
		sb.WriteString("## Callers\n\n")
		for _, c := range prog.Callers {
			sb.WriteString(fmt.Sprintf("- %s\n", c.ProgramID))
		}
		sb.WriteString("\n")
	}

	if len(prog.Callees) > 0 {
		sb.WriteString("## Callees\n\n")
		for _, c := range prog.Callees {
			sb.WriteString(fmt.Sprintf("- %s\n", c.ProgramID))
		}
		sb.WriteString("\n")
	}

	if len(prog.Copybooks) > 0 {
		sb.WriteString("## Copybooks\n\n")
		for _, c := range prog.Copybooks {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	if len(prog.Paragraphs) > 0 {
		sb.WriteString("## Paragraphs\n\n")
		sb.WriteString("| Name | Category | Description |\n|------|----------|-------------|\n")
		for _, p := range prog.Paragraphs {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", p.Name, p.Category, p.Description))
		}
		sb.WriteString("\n")
	}

	if len(prog.DataItems) > 0 {
		sb.WriteString("## Data Items\n\n")
		sb.WriteString("| Level | Name | Picture | Usage |\n|-------|------|---------|-------|\n")
		for _, d := range prog.DataItems {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", d.Level, d.Name, d.Picture, d.Usage))
		}
		sb.WriteString("\n")
	}

	outPath := filepath.Join(outputDir, fmt.Sprintf("program-%s.md", programID))
	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("writing report: %w", err)
	}
	return outPath, nil
}

// ExportCSV exports a report type as CSV for spreadsheet import.
func (s *ExportService) ExportCSV(reportType, outputDir string) (string, error) {
	r, err := s.reader()
	if err != nil {
		return "", err
	}
	ctx := context.Background()

	var filename string
	var headers []string
	var rows [][]string

	switch reportType {
	case "modernization":
		filename = "modernization-candidates.csv"
		headers = []string{"ProgramID", "Score", "Approach", "Reason"}
		candidates, err := r.ListModernizationCandidates(ctx)
		if err != nil {
			return "", err
		}
		for _, c := range candidates {
			rows = append(rows, []string{c.ProgramID, fmt.Sprintf("%.1f", c.Score), c.Approach, c.Reason})
		}

	case "risk":
		filename = "risk-programs.csv"
		headers = []string{"ProgramID", "RiskScore", "RiskType", "RiskDetails"}
		risks, err := r.ListRiskPrograms(ctx, 0)
		if err != nil {
			return "", err
		}
		for _, rp := range risks {
			rows = append(rows, []string{rp.ProgramID, fmt.Sprintf("%.1f", rp.RiskScore), rp.RiskType, rp.RiskDetails})
		}

	case "effort":
		filename = "effort-estimates.csv"
		headers = []string{"ProgramID", "TShirtSize", "ComplexityScore", "LineCount", "Paragraphs", "Copybooks", "SQL", "CICS"}
		estimates, err := r.GetEffortEstimates(ctx)
		if err != nil {
			return "", err
		}
		for _, e := range estimates {
			rows = append(rows, []string{e.ProgramID, e.TShirtSize, fmt.Sprint(e.ComplexityScore),
				fmt.Sprint(e.LineCount), fmt.Sprint(e.ParagraphCount), fmt.Sprint(e.CopybookCount),
				fmt.Sprint(e.SQLCount), fmt.Sprint(e.CICSCount)})
		}

	case "migration":
		filename = "migration-sequence.csv"
		headers = []string{"Order", "ProgramID", "Tier", "Score", "BlockedBy", "Approach"}
		steps, err := r.GetMigrationSequence(ctx)
		if err != nil {
			return "", err
		}
		for _, step := range steps {
			rows = append(rows, []string{fmt.Sprint(step.Order), step.ProgramID, step.Tier,
				fmt.Sprintf("%.1f", step.Score), strings.Join(step.BlockedBy, ";"), step.Approach})
		}

	default:
		return "", fmt.Errorf("unknown report type: %s", reportType)
	}

	outPath := filepath.Join(outputDir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating CSV: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(headers); err != nil {
		return "", err
	}
	if err := w.WriteAll(rows); err != nil {
		return "", err
	}
	w.Flush()
	return outPath, w.Error()
}
