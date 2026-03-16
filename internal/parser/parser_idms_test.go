package parser

import (
	"testing"
)

func TestParsePass1_IDMSProgram(t *testing.T) {
	jsonStr := `{
		"programId": "IDMSPROG",
		"executionMode": "IDMS_DC",
		"copyReferences": ["SUBSCHEMA-CTRL", "EMPLOYEE"],
		"callTargets": [],
		"paragraphs": ["MAIN-LOGIC", "OBTAIN-EMP"],
		"sections": [],
		"fileDefinitions": [],
		"dataItems": [],
		"conditions": [],
		"parameters": [],
		"sqlStatements": [],
		"cicsCommands": [],
		"externalInterfaces": [],
		"dbTables": [],
		"idmsSchemas": [
			{
				"schemaName": "EMPSCHM",
				"subschemaName": "EMPSS01",
				"protocolMode": "IDMS-DC"
			}
		],
		"idmsRecords": [
			{"name": "EMPLOYEE", "area": "EMP-AREA"},
			{"name": "DEPARTMENT", "area": "DEPT-AREA"}
		],
		"idmsAreas": [
			{"name": "EMP-AREA", "usageMode": "RETRIEVAL"},
			{"name": "DEPT-AREA", "usageMode": "UPDATE"}
		],
		"idmsSets": [
			{"name": "DEPT-EMP", "ownerRecord": "DEPARTMENT", "memberRecord": "EMPLOYEE"}
		]
	}`

	result, err := ParsePass1Response(jsonStr, "test.cbl")
	if err != nil {
		t.Fatalf("ParsePass1Response failed: %v", err)
	}

	if result.Programs[0].ExecutionMode != "IDMS_DC" {
		t.Errorf("expected IDMS_DC execution mode, got %s", result.Programs[0].ExecutionMode)
	}

	if len(result.IDMSSchemas) != 1 {
		t.Fatalf("expected 1 IDMS schema, got %d", len(result.IDMSSchemas))
	}
	s := result.IDMSSchemas[0]
	if s.SchemaName != "EMPSCHM" {
		t.Errorf("expected schema EMPSCHM, got %s", s.SchemaName)
	}
	if s.SubschemaName != "EMPSS01" {
		t.Errorf("expected subschema EMPSS01, got %s", s.SubschemaName)
	}
	if s.ProtocolMode != "IDMS-DC" {
		t.Errorf("expected protocol IDMS-DC, got %s", s.ProtocolMode)
	}

	if len(result.IDMSRecords) != 2 {
		t.Fatalf("expected 2 IDMS records, got %d", len(result.IDMSRecords))
	}
	if result.IDMSRecords[0].Name != "EMPLOYEE" {
		t.Errorf("expected record EMPLOYEE, got %s", result.IDMSRecords[0].Name)
	}
	if result.IDMSRecords[0].Area != "EMP-AREA" {
		t.Errorf("expected area EMP-AREA, got %s", result.IDMSRecords[0].Area)
	}

	if len(result.IDMSAreas) != 2 {
		t.Fatalf("expected 2 IDMS areas, got %d", len(result.IDMSAreas))
	}
	if result.IDMSAreas[0].UsageMode != "RETRIEVAL" {
		t.Errorf("expected usage RETRIEVAL, got %s", result.IDMSAreas[0].UsageMode)
	}

	if len(result.IDMSSets) != 1 {
		t.Fatalf("expected 1 IDMS set, got %d", len(result.IDMSSets))
	}
	set := result.IDMSSets[0]
	if set.Name != "DEPT-EMP" {
		t.Errorf("expected set DEPT-EMP, got %s", set.Name)
	}
	if set.OwnerRecord != "DEPARTMENT" || set.MemberRecord != "EMPLOYEE" {
		t.Errorf("expected owner DEPARTMENT/member EMPLOYEE, got %s/%s", set.OwnerRecord, set.MemberRecord)
	}

	// Check BINDS_TO relationship was created
	foundBindsTo := false
	for _, rel := range result.Relationships {
		if rel.Type == "BINDS_TO" && rel.FromKey == "IDMSPROG" {
			foundBindsTo = true
			break
		}
	}
	if !foundBindsTo {
		t.Error("expected BINDS_TO relationship for IDMS schema")
	}

	// Check READIES relationship was created
	foundReadies := false
	for _, rel := range result.Relationships {
		if rel.Type == "READIES" && rel.ToKey == "EMP-AREA" {
			foundReadies = true
			break
		}
	}
	if !foundReadies {
		t.Error("expected READIES relationship for IDMS area")
	}
}

func TestParsePass2_IDMSOperations(t *testing.T) {
	jsonStr := `{
		"performs": [],
		"dataFlows": [],
		"fileOperations": [],
		"sqlStatements": [],
		"cicsCommands": [],
		"dataHierarchy": [],
		"redefines": [],
		"copybookDefinitions": [],
		"annotations": [],
		"conditionalLogic": [],
		"dynamicCallResolution": [],
		"errorHandling": [
			{"paragraph": "CHECK-STATUS", "pattern": "IDMS-STATUS", "details": "Checks DB-STATUS-CODE after each DML"}
		],
		"idmsOperations": [
			{
				"verb": "BIND",
				"record": "",
				"area": "",
				"set": "",
				"calcKey": "",
				"navigation": "",
				"paragraph": "INIT-DB",
				"usageMode": ""
			},
			{
				"verb": "READY",
				"record": "",
				"area": "EMP-AREA",
				"set": "",
				"calcKey": "",
				"navigation": "",
				"paragraph": "INIT-DB",
				"usageMode": "RETRIEVAL"
			},
			{
				"verb": "OBTAIN",
				"record": "EMPLOYEE",
				"area": "",
				"set": "",
				"calcKey": "12345",
				"navigation": "CALC",
				"paragraph": "OBTAIN-EMP",
				"usageMode": ""
			},
			{
				"verb": "STORE",
				"record": "EMPLOYEE",
				"area": "",
				"set": "",
				"calcKey": "",
				"navigation": "",
				"paragraph": "ADD-EMP",
				"usageMode": ""
			},
			{
				"verb": "MODIFY",
				"record": "EMPLOYEE",
				"area": "",
				"set": "",
				"calcKey": "",
				"navigation": "",
				"paragraph": "UPDATE-EMP",
				"usageMode": ""
			},
			{
				"verb": "ERASE",
				"record": "EMPLOYEE",
				"area": "",
				"set": "",
				"calcKey": "",
				"navigation": "",
				"paragraph": "DELETE-EMP",
				"usageMode": ""
			},
			{
				"verb": "CONNECT",
				"record": "EMPLOYEE",
				"area": "",
				"set": "DEPT-EMP",
				"calcKey": "",
				"navigation": "",
				"paragraph": "LINK-DEPT",
				"usageMode": ""
			},
			{
				"verb": "DISCONNECT",
				"record": "EMPLOYEE",
				"area": "",
				"set": "DEPT-EMP",
				"calcKey": "",
				"navigation": "",
				"paragraph": "UNLINK-DEPT",
				"usageMode": ""
			}
		]
	}`

	result, err := ParsePass2Response(jsonStr, "test.cbl", "IDMSPROG")
	if err != nil {
		t.Fatalf("ParsePass2Response failed: %v", err)
	}

	if len(result.IDMSOperations) != 8 {
		t.Fatalf("expected 8 IDMS operations, got %d", len(result.IDMSOperations))
	}

	// Verify BIND
	if result.IDMSOperations[0].Verb != "BIND" {
		t.Errorf("expected BIND, got %s", result.IDMSOperations[0].Verb)
	}

	// Verify OBTAIN with CALC navigation
	obtain := result.IDMSOperations[2]
	if obtain.Verb != "OBTAIN" || obtain.Record != "EMPLOYEE" || obtain.Navigation != "CALC" || obtain.CalcKey != "12345" {
		t.Errorf("OBTAIN mismatch: %+v", obtain)
	}

	// Verify CONNECT with set
	connect := result.IDMSOperations[6]
	if connect.Verb != "CONNECT" || connect.Set != "DEPT-EMP" {
		t.Errorf("CONNECT mismatch: %+v", connect)
	}

	// Verify IDMS-STATUS error handling
	if len(result.ErrorHandlers) != 1 || result.ErrorHandlers[0].Pattern != "IDMS-STATUS" {
		t.Errorf("expected IDMS-STATUS error handler, got %+v", result.ErrorHandlers)
	}
}
