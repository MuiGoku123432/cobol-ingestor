package graph

// MergePass2Results deduplicates and merges multiple Pass2Results into one.
func MergePass2Results(results []*Pass2Result) *Pass2Result {
	if len(results) == 0 {
		return &Pass2Result{}
	}
	if len(results) == 1 {
		return results[0]
	}

	merged := &Pass2Result{
		SourceFile: results[0].SourceFile,
		ProgramID:  results[0].ProgramID,
	}

	performSeen := make(map[string]bool)
	dataFlowSeen := make(map[string]bool)
	fileOpSeen := make(map[string]bool)
	redefineSeen := make(map[string]bool)
	copybookDefSeen := make(map[string]bool)
	hierarchySeen := make(map[string]bool)
	annotationMap := make(map[string]Annotation) // paragraph → longest description
	condLogicSeen := make(map[string]bool)
	dynCallSeen := make(map[string]bool)
	errorHandlerSeen := make(map[string]bool)

	for _, r := range results {
		for _, p := range r.Performs {
			key := p.FromParagraph + "|" + p.ToParagraph + "|" + p.ThruParagraph
			if !performSeen[key] {
				performSeen[key] = true
				merged.Performs = append(merged.Performs, p)
			}
		}

		for _, d := range r.DataFlows {
			key := d.FromItem + "|" + d.ToItem + "|" + d.Context
			if !dataFlowSeen[key] {
				dataFlowSeen[key] = true
				merged.DataFlows = append(merged.DataFlows, d)
			}
		}

		for _, f := range r.FileOps {
			key := f.Operation + "|" + f.FileName + "|" + f.Paragraph
			if !fileOpSeen[key] {
				fileOpSeen[key] = true
				merged.FileOps = append(merged.FileOps, f)
			}
		}

		merged.SQLDetails = append(merged.SQLDetails, r.SQLDetails...)
		merged.CICSDetails = append(merged.CICSDetails, r.CICSDetails...)

		for _, d := range r.DataHierarchy {
			key := d.Name + "|" + d.Parent
			if !hierarchySeen[key] {
				hierarchySeen[key] = true
				merged.DataHierarchy = append(merged.DataHierarchy, d)
			}
		}

		for _, rd := range r.Redefines {
			key := rd.Item + "|" + rd.Redefines
			if !redefineSeen[key] {
				redefineSeen[key] = true
				merged.Redefines = append(merged.Redefines, rd)
			}
		}

		for _, cd := range r.CopybookDefs {
			key := cd.DataItem + "|" + cd.Copybook
			if !copybookDefSeen[key] {
				copybookDefSeen[key] = true
				merged.CopybookDefs = append(merged.CopybookDefs, cd)
			}
		}

		for _, a := range r.Annotations {
			if existing, ok := annotationMap[a.Paragraph]; ok {
				if len(a.Description) > len(existing.Description) {
					annotationMap[a.Paragraph] = a
				}
			} else {
				annotationMap[a.Paragraph] = a
			}
		}

		for _, cl := range r.ConditionalLogic {
			key := cl.Paragraph + "|" + cl.Condition
			if !condLogicSeen[key] {
				condLogicSeen[key] = true
				merged.ConditionalLogic = append(merged.ConditionalLogic, cl)
			}
		}

		for _, dc := range r.DynamicCallResolutions {
			if !dynCallSeen[dc.Variable] {
				dynCallSeen[dc.Variable] = true
				merged.DynamicCallResolutions = append(merged.DynamicCallResolutions, dc)
			}
		}

		for _, eh := range r.ErrorHandlers {
			if !errorHandlerSeen[eh.Paragraph] {
				errorHandlerSeen[eh.Paragraph] = true
				merged.ErrorHandlers = append(merged.ErrorHandlers, eh)
			}
		}
	}

	for _, a := range annotationMap {
		merged.Annotations = append(merged.Annotations, a)
	}

	return merged
}
