package neo4j

import (
	"context"
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// DetectSharedFileFlows creates DATA_FLOWS_TO relationships for programs that share files.
func (w *BatchWriter) DetectSharedFileFlows(ctx context.Context) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			"MATCH (a:Program)-[:WRITES]->(f:File)<-[:READS]-(b:Program) "+
				"WHERE a <> b "+
				"MERGE (a)-[r:DATA_FLOWS_TO]->(b) "+
				"SET r.channel = 'FILE', r.sharedResource = f.name", nil)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("detecting shared file flows: %w", err)
	}

	w.logger.Info("detected shared file data flows")
	return nil
}

// DetectSharedDB2Flows creates DATA_FLOWS_TO relationships for programs that share DB2 tables.
func (w *BatchWriter) DetectSharedDB2Flows(ctx context.Context) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			"MATCH (a:Program)-[wa:ACCESSES]->(t:DBTable)<-[rb:ACCESSES]-(b:Program) "+
				"WHERE a <> b "+
				"AND any(op IN wa.operations WHERE op IN ['INSERT', 'UPDATE']) "+
				"AND any(op IN rb.operations WHERE op = 'SELECT') "+
				"MERGE (a)-[r:DATA_FLOWS_TO]->(b) "+
				"SET r.channel = 'DB2', r.sharedResource = t.name", nil)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("detecting shared DB2 flows: %w", err)
	}

	w.logger.Info("detected shared DB2 data flows")
	return nil
}

// LinkDDCardsToFiles creates MAPS_TO_FILE relationships between DD cards and File nodes.
func (w *BatchWriter) LinkDDCardsToFiles(ctx context.Context) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			"MATCH (dd:DDCard), (f:File) "+
				"WHERE dd.ddName = f.name OR dd.dsname CONTAINS f.name "+
				"MERGE (dd)-[:MAPS_TO_FILE]->(f)", nil)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("linking DD cards to files: %w", err)
	}

	w.logger.Info("linked DD cards to files")
	return nil
}

// DetectCICSFlows creates DATA_FLOWS_TO relationships for programs linked via CICS LINK/XCTL.
// Parses ExternalInterface nodes of type CICS_LINK/CICS_XCTL to extract target program names
// from the details field (e.g., "LINK PROGRAM('CUSTRPT')").
func (w *BatchWriter) DetectCICSFlows(ctx context.Context) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	res, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MATCH (e:ExternalInterface) "+
				"WHERE e.type IN ['CICS_LINK', 'CICS_XCTL'] "+
				"AND e.details CONTAINS \"'\" "+
				"WITH e, e.programId AS callerId, "+
				"  substring(e.details, "+
				"    apoc.text.indexOf(e.details, \"'\") + 1, "+
				"    apoc.text.indexOf(e.details, \"'\", apoc.text.indexOf(e.details, \"'\") + 1) "+
				"    - apoc.text.indexOf(e.details, \"'\") - 1 "+
				"  ) AS targetName "+
				"WHERE targetName <> '' "+
				"MATCH (caller:Program {programId: callerId}) "+
				"MATCH (callee:Program {programId: targetName}) "+
				"WHERE caller <> callee "+
				"MERGE (caller)-[r:DATA_FLOWS_TO]->(callee) "+
				"SET r.channel = 'CICS_COMMAREA', r.sharedResource = 'COMMAREA' "+
				"RETURN count(r) AS cnt",
			nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if val, ok := result.Record().Get("cnt"); ok {
				if n, ok := val.(int64); ok {
					return n, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return fmt.Errorf("detecting CICS flows: %w", err)
	}

	count := int(res.(int64))
	if count > 0 {
		w.logger.Info("detected CICS LINK/XCTL data flows", zap.Int("flows", count))
	}
	return nil
}

// DetectCICSQueueFlows creates DATA_FLOWS_TO relationships for programs sharing CICS TS/TD queues.
// Matches ExternalInterface nodes where one program writes (WRITEQ) and another reads (READQ)
// the same queue name.
func (w *BatchWriter) DetectCICSQueueFlows(ctx context.Context) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	res, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			// Extract queue name and operation from CICS TS/TD interfaces
			"MATCH (e:ExternalInterface) "+
				"WHERE e.type IN ['CICS_TS', 'CICS_TD'] "+
				"AND e.details CONTAINS \"'\" "+
				"WITH e, e.programId AS pid, e.type AS qType, "+
				"  substring(e.details, "+
				"    apoc.text.indexOf(e.details, \"'\") + 1, "+
				"    apoc.text.indexOf(e.details, \"'\", apoc.text.indexOf(e.details, \"'\") + 1) "+
				"    - apoc.text.indexOf(e.details, \"'\") - 1 "+
				"  ) AS queueName, "+
				"  CASE "+
				"    WHEN toUpper(e.details) CONTAINS 'WRITEQ' THEN 'WRITE' "+
				"    WHEN toUpper(e.details) CONTAINS 'READQ' THEN 'READ' "+
				"    WHEN toUpper(e.details) CONTAINS 'DELETEQ' THEN 'WRITE' "+
				"    ELSE null "+
				"  END AS operation "+
				"WHERE queueName <> '' AND operation IS NOT NULL "+
				// Match writers to readers on same queue
				"WITH qType, queueName, "+
				"  collect(CASE WHEN operation = 'WRITE' THEN pid END) AS writers, "+
				"  collect(CASE WHEN operation = 'READ' THEN pid END) AS readers "+
				"UNWIND writers AS writerPid "+
				"UNWIND readers AS readerPid "+
				"WITH qType, queueName, writerPid, readerPid "+
				"WHERE writerPid <> readerPid AND writerPid IS NOT NULL AND readerPid IS NOT NULL "+
				"MATCH (w:Program {programId: writerPid}) "+
				"MATCH (r:Program {programId: readerPid}) "+
				"MERGE (w)-[rel:DATA_FLOWS_TO]->(r) "+
				"SET rel.channel = qType, rel.sharedResource = queueName "+
				"RETURN count(rel) AS cnt",
			nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if val, ok := result.Record().Get("cnt"); ok {
				if n, ok := val.(int64); ok {
					return n, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return fmt.Errorf("detecting CICS queue flows: %w", err)
	}

	count := int(res.(int64))
	if count > 0 {
		w.logger.Info("detected CICS TS/TD queue data flows", zap.Int("flows", count))
	}
	return nil
}

// DetectMQFlows creates DATA_FLOWS_TO relationships for programs sharing MQ queues.
// Matches ExternalInterface nodes of type MQ where one program puts (MQPUT/MQOPEN OUTPUT)
// and another gets (MQGET/MQOPEN INPUT) from the same queue.
func (w *BatchWriter) DetectMQFlows(ctx context.Context) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	res, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MATCH (e:ExternalInterface) "+
				"WHERE e.type = 'MQ' "+
				"AND e.details CONTAINS \"'\" "+
				"WITH e, e.programId AS pid, "+
				"  substring(e.details, "+
				"    apoc.text.indexOf(e.details, \"'\") + 1, "+
				"    apoc.text.indexOf(e.details, \"'\", apoc.text.indexOf(e.details, \"'\") + 1) "+
				"    - apoc.text.indexOf(e.details, \"'\") - 1 "+
				"  ) AS queueName, "+
				"  CASE "+
				"    WHEN toUpper(e.details) CONTAINS 'MQPUT' THEN 'WRITE' "+
				"    WHEN toUpper(e.details) CONTAINS 'MQOPEN' AND toUpper(e.details) CONTAINS 'OUTPUT' THEN 'WRITE' "+
				"    WHEN toUpper(e.details) CONTAINS 'MQGET' THEN 'READ' "+
				"    WHEN toUpper(e.details) CONTAINS 'MQOPEN' AND toUpper(e.details) CONTAINS 'INPUT' THEN 'READ' "+
				"    ELSE null "+
				"  END AS operation "+
				"WHERE queueName <> '' AND operation IS NOT NULL "+
				"WITH queueName, "+
				"  collect(CASE WHEN operation = 'WRITE' THEN pid END) AS writers, "+
				"  collect(CASE WHEN operation = 'READ' THEN pid END) AS readers "+
				"UNWIND writers AS writerPid "+
				"UNWIND readers AS readerPid "+
				"WITH queueName, writerPid, readerPid "+
				"WHERE writerPid <> readerPid AND writerPid IS NOT NULL AND readerPid IS NOT NULL "+
				"MATCH (w:Program {programId: writerPid}) "+
				"MATCH (r:Program {programId: readerPid}) "+
				"MERGE (w)-[rel:DATA_FLOWS_TO]->(r) "+
				"SET rel.channel = 'MQ', rel.sharedResource = queueName "+
				"RETURN count(rel) AS cnt",
			nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if val, ok := result.Record().Get("cnt"); ok {
				if n, ok := val.(int64); ok {
					return n, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return fmt.Errorf("detecting MQ flows: %w", err)
	}

	count := int(res.(int64))
	if count > 0 {
		w.logger.Info("detected MQ queue data flows", zap.Int("flows", count))
	}
	return nil
}

// DetectJCLSequenceFlows creates DATA_FLOWS_TO relationships for consecutive JCL steps
// that share datasets via DD cards. When step N writes a file and step N+M reads it
// (within the same job), a JCL_STEP flow is created between their executed programs.
func (w *BatchWriter) DetectJCLSequenceFlows(ctx context.Context) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	res, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MATCH (j:JCLJob)-[:RUNS]->(s1:JCLStep)-[:HAS_DD]->(dd1:DDCard)-[:MAPS_TO_FILE]->(f:File)<-[:MAPS_TO_FILE]-(dd2:DDCard)<-[:HAS_DD]-(s2:JCLStep)<-[:RUNS]-(j) "+
				"WHERE s1.order < s2.order "+
				"AND dd1.disposition IN ['NEW', 'MOD', 'SHR'] "+
				"AND dd2.disposition IN ['OLD', 'SHR'] "+
				"MATCH (s1)-[:EXECUTES]->(p1:Program) "+
				"MATCH (s2)-[:EXECUTES]->(p2:Program) "+
				"WHERE p1 <> p2 "+
				"MERGE (p1)-[r:DATA_FLOWS_TO]->(p2) "+
				"SET r.channel = 'JCL_STEP', r.sharedResource = f.name "+
				"RETURN count(r) AS cnt",
			nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if val, ok := result.Record().Get("cnt"); ok {
				if n, ok := val.(int64); ok {
					return n, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return fmt.Errorf("detecting JCL sequence flows: %w", err)
	}

	count := int(res.(int64))
	if count > 0 {
		w.logger.Info("detected JCL step sequence data flows", zap.Int("flows", count))
	}
	return nil
}

// QueryCICSPairsForPass4 returns CICS LINK/XCTL caller/callee pairs with COMMAREA context
// for LLM field-level mapping.
func (c *Client) QueryCICSPairsForPass4(ctx context.Context) ([]CallPairContext, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	// Find CICS LINK/XCTL pairs where both programs exist
	result, err := session.Run(ctx,
		"MATCH (e:ExternalInterface) "+
			"WHERE e.type IN ['CICS_LINK', 'CICS_XCTL'] "+
			"AND e.details CONTAINS \"'\" "+
			"WITH e.programId AS callerId, "+
			"  substring(e.details, "+
			"    apoc.text.indexOf(e.details, \"'\") + 1, "+
			"    apoc.text.indexOf(e.details, \"'\", apoc.text.indexOf(e.details, \"'\") + 1) "+
			"    - apoc.text.indexOf(e.details, \"'\") - 1 "+
			"  ) AS calleeId "+
			"WHERE calleeId <> '' "+
			"MATCH (caller:Program {programId: callerId}) "+
			"MATCH (callee:Program {programId: calleeId}) "+
			"RETURN DISTINCT callerId, calleeId",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying CICS pairs: %w", err)
	}

	type pair struct{ caller, callee string }
	var pairs []pair
	for result.Next(ctx) {
		rec := result.Record()
		pairs = append(pairs, pair{
			caller: getStr(rec, "callerId"),
			callee: getStr(rec, "calleeId"),
		})
	}

	var contexts []CallPairContext
	for _, p := range pairs {
		cpc := CallPairContext{CallerID: p.caller, CalleeID: p.callee}

		// Get caller's WORKING-STORAGE data items (COMMAREA copy)
		callerRes, err := session.Run(ctx,
			"MATCH (d:DataItem {programId: $pid}) WHERE d.level IN [1, 77] "+
				"RETURN d.name AS name, d.picture AS picture",
			map[string]any{"pid": p.caller})
		if err == nil {
			for callerRes.Next(ctx) {
				rec := callerRes.Record()
				cpc.CallerFields = append(cpc.CallerFields, FieldContext{
					Name:    getStr(rec, "name"),
					Picture: getStr(rec, "picture"),
				})
			}
		}

		// Get callee's LINKAGE SECTION (DFHCOMMAREA layout)
		calleeRes, err := session.Run(ctx,
			"MATCH (d:DataItem {programId: $pid}) "+
				"WHERE d.section = 'LINKAGE' OR d.level IN [1, 77] "+
				"RETURN d.name AS name, d.picture AS picture",
			map[string]any{"pid": p.callee})
		if err == nil {
			for calleeRes.Next(ctx) {
				rec := calleeRes.Record()
				cpc.CalleeParams = append(cpc.CalleeParams, FieldContext{
					Name:    getStr(rec, "name"),
					Picture: getStr(rec, "picture"),
				})
			}
		}

		if len(cpc.CalleeParams) > 0 {
			contexts = append(contexts, cpc)
		}
	}

	return contexts, nil
}

// QuerySharedFilePairsWithCopybooks returns shared-file writer/reader pairs that both include
// a common copybook (the file record layout), for LLM field-level enrichment.
func (c *Client) QuerySharedFilePairsWithCopybooks(ctx context.Context) ([]SharedFilePairContext, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (writer:Program)-[:WRITES]->(f:File)<-[:READS]-(reader:Program) "+
			"WHERE writer <> reader "+
			"MATCH (writer)-[:INCLUDES]->(cb:Copybook)<-[:INCLUDES]-(reader) "+
			"RETURN DISTINCT writer.programId AS writerId, reader.programId AS readerId, "+
			"  f.name AS fileName, cb.name AS copybookName",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying shared file pairs with copybooks: %w", err)
	}

	var pairs []SharedFilePairContext
	for result.Next(ctx) {
		rec := result.Record()
		pairs = append(pairs, SharedFilePairContext{
			WriterID:     getStr(rec, "writerId"),
			ReaderID:     getStr(rec, "readerId"),
			FileName:     getStr(rec, "fileName"),
			CopybookName: getStr(rec, "copybookName"),
		})
	}
	return pairs, nil
}

// SharedFilePairContext holds context for a shared file pair with a common copybook.
type SharedFilePairContext struct {
	WriterID     string
	ReaderID     string
	FileName     string
	CopybookName string
}

// CallPairContext holds context for a CALLS relationship to be analyzed for LINKAGE mapping.
type CallPairContext struct {
	CallerID      string
	CalleeID      string
	CallerFields  []FieldContext
	CalleeParams  []FieldContext
}

// FieldContext holds field info for cross-program mapping.
type FieldContext struct {
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Direction string `json:"direction,omitempty"`
	MovedFrom []string `json:"movedFrom,omitempty"`
}

// QueryCallPairsForPass4 returns call pairs with their parameter context for LINKAGE analysis.
func (c *Client) QueryCallPairsForPass4(ctx context.Context) ([]CallPairContext, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	// Get all CALLS relationships
	result, err := session.Run(ctx,
		"MATCH (caller:Program)-[:CALLS]->(callee:Program) "+
			"RETURN DISTINCT caller.programId AS callerId, callee.programId AS calleeId", nil)
	if err != nil {
		return nil, fmt.Errorf("querying call pairs: %w", err)
	}

	type pair struct{ caller, callee string }
	var pairs []pair
	for result.Next(ctx) {
		rec := result.Record()
		pairs = append(pairs, pair{
			caller: getStr(rec, "callerId"),
			callee: getStr(rec, "calleeId"),
		})
	}

	var contexts []CallPairContext
	for _, p := range pairs {
		cpc := CallPairContext{CallerID: p.caller, CalleeID: p.callee}

		// Get caller's data items (potential CALL USING fields)
		callerRes, err := session.Run(ctx,
			"MATCH (d:DataItem {programId: $pid}) WHERE d.level IN [1, 77] "+
				"RETURN d.name AS name, d.picture AS picture",
			map[string]any{"pid": p.caller})
		if err == nil {
			for callerRes.Next(ctx) {
				rec := callerRes.Record()
				cpc.CallerFields = append(cpc.CallerFields, FieldContext{
					Name:    getStr(rec, "name"),
					Picture: getStr(rec, "picture"),
				})
			}
		}

		// Get callee's LINKAGE parameters
		calleeRes, err := session.Run(ctx,
			"MATCH (p:Parameter {programId: $pid}) "+
				"RETURN p.name AS name, p.direction AS direction",
			map[string]any{"pid": p.callee})
		if err == nil {
			for calleeRes.Next(ctx) {
				rec := calleeRes.Record()
				cpc.CalleeParams = append(cpc.CalleeParams, FieldContext{
					Name:      getStr(rec, "name"),
					Direction: getStr(rec, "direction"),
				})
			}
		}

		// Only include pairs where callee has LINKAGE parameters
		if len(cpc.CalleeParams) > 0 {
			contexts = append(contexts, cpc)
		}
	}

	return contexts, nil
}

// WritePass4Result writes cross-program data flow relationships to Neo4j.
func (w *BatchWriter) WritePass4Result(ctx context.Context, result *graph.Pass4Result) error {
	if len(result.Flows) == 0 {
		return nil
	}

	var rows []map[string]any
	for _, f := range result.Flows {
		fieldsJSON, _ := json.Marshal(f.Fields)
		rows = append(rows, map[string]any{
			"fromPid":        f.FromProgram,
			"toPid":          f.ToProgram,
			"channel":        f.Channel,
			"fields":         string(fieldsJSON),
			"sharedResource": f.SharedResource,
		})
	}

	if err := w.batchUpdate(ctx,
		"UNWIND $rows AS row "+
			"MATCH (a:Program {programId: row.fromPid}) "+
			"MATCH (b:Program {programId: row.toPid}) "+
			"MERGE (a)-[r:DATA_FLOWS_TO {channel: row.channel}]->(b) "+
			"SET r.fields = row.fields, r.sharedResource = row.sharedResource",
		rows); err != nil {
		return fmt.Errorf("writing pass4 flows: %w", err)
	}

	w.logger.Info("wrote pass 4 results", zap.Int("flows", len(result.Flows)))
	return nil
}

// WritePass4FieldMappings creates LINKAGE_MAPS_TO edges between DataItem and Parameter nodes.
func (w *BatchWriter) WritePass4FieldMappings(ctx context.Context, result *graph.Pass4Result) error {
	var rows []map[string]any
	for _, flow := range result.Flows {
		if flow.Channel != "LINKAGE" {
			continue
		}
		for _, fp := range flow.Fields {
			rows = append(rows, map[string]any{
				"callerPid":   flow.FromProgram,
				"calleePid":   flow.ToProgram,
				"sourceField": fp.SourceField,
				"targetField": fp.TargetField,
				"transform":   fp.Transform,
			})
		}
	}

	if len(rows) == 0 {
		return nil
	}

	if err := w.batchUpdate(ctx,
		"UNWIND $rows AS row "+
			"MATCH (src:DataItem {programId: row.callerPid, name: row.sourceField}) "+
			"MATCH (tgt:Parameter {programId: row.calleePid, name: row.targetField}) "+
			"MERGE (src)-[r:LINKAGE_MAPS_TO]->(tgt) "+
			"SET r.transform = row.transform",
		rows); err != nil {
		return fmt.Errorf("writing pass4 field mappings: %w", err)
	}

	w.logger.Info("wrote pass 4 field mappings", zap.Int("mappings", len(rows)))
	return nil
}

// GetCrossProgramDataFlow returns all cross-program data flows for a program.
func (c *Client) GetCrossProgramDataFlow(ctx context.Context, programID string) ([]CrossProgramFlowInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	params := map[string]any{"pid": programID}
	var items []CrossProgramFlowInfo

	// Outbound flows
	outRes, err := session.Run(ctx,
		"MATCH (a:Program {programId: $pid})-[r:DATA_FLOWS_TO]->(b:Program) "+
			"RETURN b.programId AS otherProgram, r.channel AS channel, r.fields AS fields, r.sharedResource AS sharedResource, 'outbound' AS direction",
		params)
	if err != nil {
		return nil, fmt.Errorf("outbound data flow query: %w", err)
	}
	for outRes.Next(ctx) {
		rec := outRes.Record()
		items = append(items, CrossProgramFlowInfo{
			ProgramID:      programID,
			OtherProgram:   getStr(rec, "otherProgram"),
			Channel:        getStr(rec, "channel"),
			Direction:      "outbound",
			Fields:         getStr(rec, "fields"),
			SharedResource: getStr(rec, "sharedResource"),
		})
	}

	// Inbound flows
	inRes, err := session.Run(ctx,
		"MATCH (a:Program)-[r:DATA_FLOWS_TO]->(b:Program {programId: $pid}) "+
			"RETURN a.programId AS otherProgram, r.channel AS channel, r.fields AS fields, r.sharedResource AS sharedResource, 'inbound' AS direction",
		params)
	if err != nil {
		return nil, fmt.Errorf("inbound data flow query: %w", err)
	}
	for inRes.Next(ctx) {
		rec := inRes.Record()
		items = append(items, CrossProgramFlowInfo{
			ProgramID:      programID,
			OtherProgram:   getStr(rec, "otherProgram"),
			Channel:        getStr(rec, "channel"),
			Direction:      "inbound",
			Fields:         getStr(rec, "fields"),
			SharedResource: getStr(rec, "sharedResource"),
		})
	}

	return items, nil
}

// TraceFieldImpact traces a field downstream through CALLS + LINKAGE data flows.
func (c *Client) TraceFieldImpact(ctx context.Context, programID, fieldName string) ([]FieldImpactInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	// Trace intra-program MOVES_TO, then cross-program DATA_FLOWS_TO
	result, err := session.Run(ctx,
		// First, find intra-program data flow
		"MATCH (src:DataItem {programId: $pid, name: $field}) "+
			"OPTIONAL MATCH path = (src)-[:MOVES_TO*1..10]->(dst:DataItem {programId: $pid}) "+
			"WITH $pid AS startPid, $field AS startField, collect(DISTINCT dst.name) AS intraTargets "+
			// Then find cross-program flows
			"OPTIONAL MATCH (prog:Program {programId: startPid})-[r:DATA_FLOWS_TO]->(callee:Program) "+
			"RETURN startPid, startField, intraTargets, "+
			"       collect(DISTINCT {callee: callee.programId, channel: r.channel, fields: r.fields}) AS crossFlows",
		map[string]any{"pid": programID, "field": fieldName})
	if err != nil {
		return nil, fmt.Errorf("field impact query: %w", err)
	}

	var items []FieldImpactInfo
	if result.Next(ctx) {
		rec := result.Record()
		info := FieldImpactInfo{
			ProgramID:    programID,
			FieldName:    fieldName,
		}

		// Intra-program targets
		if val, ok := rec.Get("intraTargets"); ok && val != nil {
			if arr, ok := val.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						info.IntraTargets = append(info.IntraTargets, s)
					}
				}
			}
		}

		// Cross-program flows
		if val, ok := rec.Get("crossFlows"); ok && val != nil {
			if arr, ok := val.([]any); ok {
				for _, v := range arr {
					if m, ok := v.(map[string]any); ok {
						callee, _ := m["callee"].(string)
						channel, _ := m["channel"].(string)
						fields, _ := m["fields"].(string)
						if callee != "" {
							info.CrossFlows = append(info.CrossFlows, CrossFlowTarget{
								Callee:  callee,
								Channel: channel,
								Fields:  fields,
							})
						}
					}
				}
			}
		}

		items = append(items, info)
	}

	return items, nil
}

// GetSharedDataChannels lists all shared files/tables with the programs that use them.
func (c *Client) GetSharedDataChannels(ctx context.Context) ([]SharedDataChannelInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	var items []SharedDataChannelInfo

	// Shared files
	fileRes, err := session.Run(ctx,
		"MATCH (a:Program)-[w:WRITES]->(f:File)<-[r:READS]-(b:Program) "+
			"WHERE a <> b "+
			"RETURN f.name AS resource, 'FILE' AS channel, "+
			"       collect(DISTINCT a.programId) AS writers, "+
			"       collect(DISTINCT b.programId) AS readers", nil)
	if err == nil {
		for fileRes.Next(ctx) {
			rec := fileRes.Record()
			items = append(items, SharedDataChannelInfo{
				Resource: getStr(rec, "resource"),
				Channel:  "FILE",
				Writers:  getStringArray(rec, "writers"),
				Readers:  getStringArray(rec, "readers"),
			})
		}
	}

	// Shared DB2 tables
	tableRes, err := session.Run(ctx,
		"MATCH (a:Program)-[wa:ACCESSES]->(t:DBTable)<-[rb:ACCESSES]-(b:Program) "+
			"WHERE a <> b "+
			"AND any(op IN wa.operations WHERE op IN ['INSERT', 'UPDATE']) "+
			"AND any(op IN rb.operations WHERE op = 'SELECT') "+
			"RETURN t.name AS resource, 'DB2' AS channel, "+
			"       collect(DISTINCT a.programId) AS writers, "+
			"       collect(DISTINCT b.programId) AS readers", nil)
	if err == nil {
		for tableRes.Next(ctx) {
			rec := tableRes.Record()
			items = append(items, SharedDataChannelInfo{
				Resource: getStr(rec, "resource"),
				Channel:  "DB2",
				Writers:  getStringArray(rec, "writers"),
				Readers:  getStringArray(rec, "readers"),
			})
		}
	}

	return items, nil
}

func getStringArray(rec *neo4j.Record, key string) []string {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return nil
	}
	arr, ok := val.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
