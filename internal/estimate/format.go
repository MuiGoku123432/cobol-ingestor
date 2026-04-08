package estimate

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// PrintTable writes a human-readable cost estimate table to w.
func PrintTable(w io.Writer, r *Result) {
	fmt.Fprintf(w, "\nCost Estimate for: %s\n", r.SourceDir)
	fmt.Fprintf(w, "Provider: %s\n", r.Provider)
	fmt.Fprintf(w, "%s\n\n", divider(60))

	fmt.Fprintf(w, "Files scanned: %d total (%d COBOL, %d copybooks, %d JCL",
		r.Files.Total, r.Files.COBOL, r.Files.Copybook, r.Files.JCL)
	if r.Files.Pending > 0 {
		fmt.Fprintf(w, ", %d pending classification (counted as COBOL)", r.Files.Pending)
	}
	fmt.Fprintln(w, ")")

	if r.Cache.Pass1 > 0 || r.Cache.Pass2 > 0 {
		fmt.Fprintf(w, "Cache hits:    Pass 1: %d skipped, Pass 2: %d skipped\n", r.Cache.Pass1, r.Cache.Pass2)
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Always show both Anthropic and Copilot columns
	fmt.Fprintln(tw, "Pass\tModel\tRequests\tInput Tokens\tAnthropic (typ)\tAnthropic (max)\tCopilot")
	fmt.Fprintln(tw, "────────────────────────\t───────\t────────\t────────────\t───────────────\t───────────────\t───────")

	hasHeuristics := false
	for _, p := range r.Passes {
		if p.Requests == 0 {
			continue
		}
		label := p.Name
		if !p.Deterministic {
			label += " *"
			hasHeuristics = true
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			label,
			p.Model,
			formatInt(p.Requests),
			formatInt(p.InputTokens),
			formatCost(p.InputCost+p.OutputCostLow),
			formatCost(p.InputCost+p.OutputCostHigh),
			formatCost(p.CopilotCost),
		)
	}

	if r.Scanner.Requests > 0 {
		fmt.Fprintf(tw, "Scanner Classification\tsonnet\t%s\t%s\t%s\t%s\t%s\n",
			formatInt(r.Scanner.Requests),
			formatInt(r.Scanner.InputTokens),
			formatCost(r.Scanner.InputCost+r.Scanner.OutputCostLow),
			formatCost(r.Scanner.InputCost+r.Scanner.OutputCostHigh),
			formatCost(r.Scanner.CopilotCost),
		)
	}

	// Totals row
	fmt.Fprintln(tw, "────────────────────────\t───────\t────────\t────────────\t───────────────\t───────────────\t───────")
	fmt.Fprintf(tw, "TOTAL\t\t%s\t%s\t%s\t%s\t%s\n",
		formatInt(r.Total.Requests),
		formatInt(r.Total.InputTokens),
		formatCost(r.Total.InputCost+r.Total.OutputCostLow),
		formatCost(r.Total.InputCost+r.Total.OutputCostHigh),
		formatCost(r.Total.CopilotCost),
	)

	tw.Flush()

	if hasHeuristics {
		fmt.Fprintln(w, "\n* = heuristic estimate (actual depends on analysis results from earlier passes)")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Anthropic API: Opus $15/$75 per MTok (in/out), Sonnet $3/$15 per MTok (in/out)")
	fmt.Fprintf(w, "Copilot:       $%.2f/premium request (Opus x%.0f, Sonnet x%.0f) — includes %.1fx retry multiplier for truncation resends\n",
		DefaultCopilotPricing.BasePerRequest,
		DefaultCopilotPricing.Multiplier["opus"],
		DefaultCopilotPricing.Multiplier["sonnet"],
		truncationRetryMultiplier,
	)
}

// PrintJSON writes a JSON cost estimate to w.
func PrintJSON(w io.Writer, r *Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatCost(c float64) string {
	if c == 0 {
		return "$0.00"
	}
	if c < 0.01 {
		return "< $0.01"
	}
	return fmt.Sprintf("$%.2f", c)
}

func divider(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '='
	}
	return string(b)
}
