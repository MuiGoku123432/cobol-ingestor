package graph

// MergePass1Results merges multiple Pass1Results (from multi-chunk analysis of a single file)
// into a single Pass1Result. Deduplicates by name/key for each entity type.
func MergePass1Results(results []*Pass1Result) *Pass1Result {
	if len(results) == 0 {
		return &Pass1Result{}
	}
	if len(results) == 1 {
		return results[0]
	}

	merged := &Pass1Result{
		SourceFile: results[0].SourceFile,
	}

	programSeen := make(map[string]bool)
	paragraphSeen := make(map[string]bool)
	sectionSeen := make(map[string]bool)
	copybookSeen := make(map[string]bool)
	dataItemSeen := make(map[string]bool)
	conditionSeen := make(map[string]bool)
	paramSeen := make(map[string]bool)
	fileDefSeen := make(map[string]bool)
	sqlSeen := make(map[string]bool)
	cicsSeen := make(map[string]bool)
	extIfSeen := make(map[string]bool)
	relSeen := make(map[string]bool)

	for _, r := range results {
		for _, p := range r.Programs {
			if !programSeen[p.ProgramID] {
				programSeen[p.ProgramID] = true
				merged.Programs = append(merged.Programs, p)
			}
		}
		for _, p := range r.Paragraphs {
			if !paragraphSeen[p.Name] {
				paragraphSeen[p.Name] = true
				merged.Paragraphs = append(merged.Paragraphs, p)
			}
		}
		for _, s := range r.Sections {
			if !sectionSeen[s.Name] {
				sectionSeen[s.Name] = true
				merged.Sections = append(merged.Sections, s)
			}
		}
		for _, c := range r.Copybooks {
			if !copybookSeen[c.Name] {
				copybookSeen[c.Name] = true
				merged.Copybooks = append(merged.Copybooks, c)
			}
		}
		for _, d := range r.DataItems {
			if !dataItemSeen[d.FQN] {
				dataItemSeen[d.FQN] = true
				merged.DataItems = append(merged.DataItems, d)
			}
		}
		for _, c := range r.Conditions {
			if !conditionSeen[c.FQN] {
				conditionSeen[c.FQN] = true
				merged.Conditions = append(merged.Conditions, c)
			}
		}
		for _, p := range r.Parameters {
			if !paramSeen[p.FQN] {
				paramSeen[p.FQN] = true
				merged.Parameters = append(merged.Parameters, p)
			}
		}
		for _, f := range r.FileDefs {
			if !fileDefSeen[f.Name] {
				fileDefSeen[f.Name] = true
				merged.FileDefs = append(merged.FileDefs, f)
			}
		}
		for _, s := range r.SQLStatements {
			if !sqlSeen[s.ID] {
				sqlSeen[s.ID] = true
				merged.SQLStatements = append(merged.SQLStatements, s)
			}
		}
		for _, c := range r.CICSTxns {
			if !cicsSeen[c.ID] {
				cicsSeen[c.ID] = true
				merged.CICSTxns = append(merged.CICSTxns, c)
			}
		}
		for _, e := range r.ExternalInterfaces {
			if !extIfSeen[e.ID] {
				extIfSeen[e.ID] = true
				merged.ExternalInterfaces = append(merged.ExternalInterfaces, e)
			}
		}
		for _, rel := range r.Relationships {
			key := string(rel.Type) + "|" + rel.FromLabel + "|" + rel.FromKey + "|" + rel.ToLabel + "|" + rel.ToKey
			if !relSeen[key] {
				relSeen[key] = true
				merged.Relationships = append(merged.Relationships, rel)
			}
		}
	}

	return merged
}
