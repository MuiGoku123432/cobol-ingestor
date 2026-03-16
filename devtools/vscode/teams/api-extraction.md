---
description: API extraction from COBOL with 4 specialist agents
mode: agent
tools: ["cobol-graph"]
---

# API Extraction Team

Extract modern API services from COBOL programs by identifying service boundaries, mapping data schemas, and implementing REST/gRPC endpoints that replicate legacy behavior.

## Strategy

COBOL programs with well-defined LINKAGE SECTIONs are natural API candidates. Each program's parameters become request/response contracts, business domains define service boundaries, and the knowledge graph ensures complete coverage of all data transformations and error paths.

## Agent Roles

### 1. API Designer

Define service contracts and boundaries from COBOL program interfaces.

**MCP Tools**: `get_program_parameters`, `get_data_items`, `get_data_hierarchy`, `get_type_mappings`, `list_business_domains`, `get_business_domain`

**Tasks**:
- Analyze LINKAGE SECTION parameters to define request/response schemas
- Group programs by business domain to establish service boundaries
- Map COBOL data hierarchies to nested API resource models
- Use type mappings to determine appropriate field types in contracts
- Produce OpenAPI or protobuf specifications for each service
- Identify shared data structures that become common schema definitions

### 2. Implementer

Build endpoint logic that replicates COBOL program behavior.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_program_sql`, `get_program_cics`, `get_program_error_handlers`

**Tasks**:
- Retrieve full program source and trace paragraph execution flow
- Translate COBOL conditional logic into endpoint business rules
- Convert embedded SQL operations to repository/data access patterns
- Map CICS transaction handling to modern request lifecycle management
- Implement error handlers matching COBOL error paths and return codes
- Ensure each endpoint satisfies the API Designer's contract specification

### 3. Data Mapper

Transform COBOL data structures into modern schemas and manage data flow.

**MCP Tools**: `get_data_items`, `get_data_hierarchy`, `get_type_mappings`, `get_copybook_structure`, `get_data_flow`, `get_cross_program_data_flow`

**Tasks**:
- Map COBOL PIC clauses and USAGE types to modern data types
- Flatten or restructure COBOL record hierarchies for API consumption
- Trace data flow through programs to ensure no transformations are missed
- Identify shared copybook structures that become common DTOs
- Handle cross-program data flows that span multiple service boundaries
- Document encoding conversions (EBCDIC, packed decimal, COMP fields)

### 4. Integration Tester

Validate API endpoints produce identical results to COBOL programs.

**MCP Tools**: `get_program_parameters`, `get_program_conditions`, `get_program_conditional_logic`, `get_program_external_interfaces`, `get_shared_data_channels`

**Tasks**:
- Generate request/response test cases from LINKAGE parameter definitions
- Create tests for every conditional branch identified in the graph
- Test external interface compatibility (file I/O, DB2, CICS)
- Verify shared data channels produce consistent results across services
- Build contract tests ensuring API schema compliance
- Run end-to-end tests comparing API output against COBOL output

## Workflow

1. **API Designer** groups programs by business domain and produces service contracts with OpenAPI/protobuf specs
2. **Data Mapper** transforms COBOL data hierarchies into modern schemas, producing type mapping documentation and shared DTOs
3. **Implementer** builds endpoint logic for each service, consuming contracts from API Designer and schemas from Data Mapper
4. **Integration Tester** validates each endpoint against COBOL behavior and contract specifications
5. Deploy services alongside COBOL with a routing layer to shift traffic incrementally

## Handoff Protocol

- API Designer outputs service contracts and boundary definitions consumed by Implementer and Integration Tester
- Data Mapper outputs schema definitions and type mappings consumed by Implementer
- Integration Tester reports failures back to Implementer with specific parameter and condition details
- Each service is independently deployable once tests pass
