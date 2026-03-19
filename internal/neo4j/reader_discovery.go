package neo4j

import (
	"context"
	"fmt"
)

// ListJCLJobs returns all JCL jobs with step counts.
func (c *Client) ListJCLJobs(ctx context.Context) ([]JCLJobInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (j:JCLJob)"+codebaseWhere("j", c.codebase)+" "+
			"OPTIONAL MATCH (s:JCLStep)-[:STEP_OF]->(j) "+
			"RETURN j.jobName AS jobName, j.class AS class, j.msgclass AS msgclass, count(s) AS stepCount "+
			"ORDER BY j.jobName", nil)
	if err != nil {
		return nil, fmt.Errorf("listing JCL jobs: %w", err)
	}

	var items []JCLJobInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, JCLJobInfo{
			JobName:   getStr(rec, "jobName"),
			Class:     getStr(rec, "class"),
			MsgClass:  getStr(rec, "msgclass"),
			StepCount: int(getInt64(rec, "stepCount")),
		})
	}
	return items, nil
}

// GetJCLJob returns full job detail including steps, programs, and datasets.
func (c *Client) GetJCLJob(ctx context.Context, jobName string) (*JCLJobDetail, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	params := map[string]any{"jobName": jobName}

	// Get job
	jobRes, err := session.Run(ctx,
		"MATCH (j:JCLJob {jobName: $jobName}) RETURN j.class AS class, j.msgclass AS msgclass", params)
	if err != nil {
		return nil, fmt.Errorf("getting JCL job: %w", err)
	}
	if !jobRes.Next(ctx) {
		return nil, nil
	}

	rec := jobRes.Record()
	detail := &JCLJobDetail{
		JobName:  jobName,
		Class:    getStr(rec, "class"),
		MsgClass: getStr(rec, "msgclass"),
	}

	// Get steps with DD cards
	stepRes, err := session.Run(ctx,
		"MATCH (s:JCLStep)-[r:STEP_OF]->(j:JCLJob {jobName: $jobName}) "+
			"OPTIONAL MATCH (s)-[:USES_DATASET]->(dd:DDCard) "+
			"RETURN s.stepName AS stepName, s.program AS program, s.proc AS proc, "+
			"       s.cond AS cond, r.order AS ord, "+
			"       collect({ddName: dd.ddName, dsname: dd.dsname, disp: dd.disp, isInput: dd.isInput, isOutput: dd.isOutput}) AS ddCards "+
			"ORDER BY r.order", params)
	if err == nil {
		for stepRes.Next(ctx) {
			r := stepRes.Record()
			step := JCLStepInfo{
				StepName: getStr(r, "stepName"),
				Program:  getStr(r, "program"),
				Proc:     getStr(r, "proc"),
				Cond:     getStr(r, "cond"),
				Order:    int(getInt64(r, "ord")),
			}

			if val, ok := r.Get("ddCards"); ok && val != nil {
				if arr, ok := val.([]any); ok {
					for _, v := range arr {
						if m, ok := v.(map[string]any); ok {
							ddName, _ := m["ddName"].(string)
							if ddName == "" {
								continue
							}
							dsname, _ := m["dsname"].(string)
							disp, _ := m["disp"].(string)
							isInput, _ := m["isInput"].(bool)
							isOutput, _ := m["isOutput"].(bool)
							step.DDCards = append(step.DDCards, DDCardInfo{
								DDName:   ddName,
								DSName:   dsname,
								Disp:     disp,
								IsInput:  isInput,
								IsOutput: isOutput,
							})
						}
					}
				}
			}

			detail.Steps = append(detail.Steps, step)
		}
	}

	return detail, nil
}

// GetProgramJCL returns which JCL jobs/steps invoke a program.
func (c *Client) GetProgramJCL(ctx context.Context, programID string) (*ProgramJCLInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (s:JCLStep)-[:RUNS]->(p:Program {programId: $pid}) "+
			"OPTIONAL MATCH (s)-[:STEP_OF]->(j:JCLJob) "+
			"RETURN DISTINCT j.jobName AS jobName, s.stepName AS stepName",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("program JCL query: %w", err)
	}

	info := &ProgramJCLInfo{ProgramID: programID}
	jobSet := map[string]bool{}
	for result.Next(ctx) {
		rec := result.Record()
		job := getStr(rec, "jobName")
		step := getStr(rec, "stepName")
		if job != "" && !jobSet[job] {
			info.Jobs = append(info.Jobs, job)
			jobSet[job] = true
		}
		if step != "" {
			info.Steps = append(info.Steps, step)
		}
	}

	return info, nil
}

// GetDatasetUsage returns which jobs read/write a dataset.
func (c *Client) GetDatasetUsage(ctx context.Context, dsname string) ([]DatasetUsageInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (dd:DDCard) WHERE dd.dsname CONTAINS $dsname "+
			"OPTIONAL MATCH (s:JCLStep)-[:USES_DATASET]->(dd) "+
			"OPTIONAL MATCH (s)-[:STEP_OF]->(j:JCLJob) "+
			"RETURN dd.dsname AS dsname, dd.isInput AS isInput, dd.isOutput AS isOutput, "+
			"       collect(DISTINCT j.jobName) AS jobs, collect(DISTINCT s.stepName) AS steps",
		map[string]any{"dsname": dsname})
	if err != nil {
		return nil, fmt.Errorf("dataset usage query: %w", err)
	}

	var items []DatasetUsageInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DatasetUsageInfo{
			DSName:   getStr(rec, "dsname"),
			IsInput:  getBool(rec, "isInput"),
			IsOutput: getBool(rec, "isOutput"),
			Jobs:     getStringArray(rec, "jobs"),
			Steps:    getStringArray(rec, "steps"),
		})
	}
	return items, nil
}

// ListDBTables returns all database tables with access counts.
func (c *Client) ListDBTables(ctx context.Context) ([]DBTableInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (t:DBTable) WHERE 1=1"+codebasesWhereAnd("t", c.codebase)+" "+
			"OPTIONAL MATCH (p:Program)-[:ACCESSES]->(t) "+
			"RETURN t.name AS name, t.schema AS schema, count(p) AS accessCount "+
			"ORDER BY t.name", nil)
	if err != nil {
		return nil, fmt.Errorf("listing DB tables: %w", err)
	}

	var items []DBTableInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DBTableInfo{
			Name:        getStr(rec, "name"),
			Schema:      getStr(rec, "schema"),
			AccessCount: int(getInt64(rec, "accessCount")),
		})
	}
	return items, nil
}

// GetTableUsage returns programs that access a table with operations and columns.
func (c *Client) GetTableUsage(ctx context.Context, tableName string) (*TableUsageInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program)-[r:ACCESSES]->(t:DBTable {name: $name}) "+
			"RETURN p.programId AS programId, r.operations AS operations, r.columns AS columns "+
			"ORDER BY p.programId",
		map[string]any{"name": tableName})
	if err != nil {
		return nil, fmt.Errorf("table usage query: %w", err)
	}

	usage := &TableUsageInfo{TableName: tableName}
	for result.Next(ctx) {
		rec := result.Record()
		access := TableAccessInfo{
			ProgramID:  getStr(rec, "programId"),
			Operations: getStringArray(rec, "operations"),
			Columns:    getStringArray(rec, "columns"),
		}
		usage.Programs = append(usage.Programs, access)
	}
	return usage, nil
}

// GetProgramTableAccess returns tables accessed by a program.
func (c *Client) GetProgramTableAccess(ctx context.Context, programID string) (*ProgramTableAccessInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program {programId: $pid})-[r:ACCESSES]->(t:DBTable) "+
			"RETURN t.name AS name, r.operations AS operations, r.columns AS columns "+
			"ORDER BY t.name",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("program table access query: %w", err)
	}

	info := &ProgramTableAccessInfo{ProgramID: programID}
	for result.Next(ctx) {
		rec := result.Record()
		info.Tables = append(info.Tables, TableAccessDetail{
			Name:       getStr(rec, "name"),
			Operations: getStringArray(rec, "operations"),
			Columns:    getStringArray(rec, "columns"),
		})
	}
	return info, nil
}
