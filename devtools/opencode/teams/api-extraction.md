# API Extraction Team

Extract COBOL program functionality as modern API services. Each business capability exposed through COBOL LINKAGE SECTIONs, CICS transactions, and batch interfaces becomes a well-defined API endpoint with typed schemas derived from the knowledge graph.

## Strategy

Analyze COBOL programs by business domain, extract their input/output contracts from LINKAGE SECTIONs and data hierarchies, and reimplement each as a standalone API service. The knowledge graph provides the type mappings, data flows, and interface definitions needed to produce accurate service boundaries without manual reverse engineering.

## Agent Roles

### API Designer

Define service boundaries, endpoints, and schemas from COBOL program interfaces.

**MCP Tools**: `get_program_parameters`, `get_data_items`, `get_data_hierarchy`, `get_type_mappings`, `list_business_domains`, `get_business_domain`

**Responsibilities**:
- Group programs into service boundaries by business domain
- Extract input/output schemas from LINKAGE SECTION parameters and data hierarchies
- Map COBOL PIC clauses to modern types using type mappings
- Define REST/gRPC endpoint contracts for each program's capabilities
- Identify shared data structures that become common schema definitions

### Implementer

Build the API service logic from COBOL program behavior.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_program_sql`, `get_program_cics`, `get_program_error_handlers`

**Responsibilities**:
- Retrieve full program source and paragraph execution flow
- Convert paragraph logic to service handler methods
- Translate embedded SQL to modern data access patterns
- Convert CICS transaction logic to stateless request handling
- Implement error handlers matching COBOL error paths

### Data Mapper

Map COBOL data structures to modern schemas and handle data transformation.

**MCP Tools**: `get_data_items`, `get_data_hierarchy`, `get_type_mappings`, `get_copybook_structure`, `get_data_flow`, `get_cross_program_data_flow`

**Responsibilities**:
- Build complete data models from copybook structures and data hierarchies
- Create transformation layers between COBOL packed/zoned formats and modern types
- Trace data flows to identify all producers and consumers of each structure
- Detect cross-program data dependencies that require shared schemas
- Produce mapping specifications for ETL between legacy and modern stores

### Integration Tester

Verify API services match COBOL program behavior end-to-end.

**MCP Tools**: `get_program_parameters`, `get_program_conditions`, `get_program_conditional_logic`, `get_program_external_interfaces`, `get_shared_data_channels`

**Responsibilities**:
- Generate contract tests from LINKAGE parameter definitions
- Create scenario tests covering all conditional logic branches
- Test external interface compatibility (files, queues, databases)
- Verify shared data channel behavior matches cross-program expectations
- Build integration suites that validate API responses against COBOL outputs

## Workflow

1. API Designer groups programs by business domain and defines service boundaries with endpoint contracts
2. Data Mapper builds schema definitions from copybook structures and data hierarchies
3. Implementer builds service handlers from paragraph flow and program logic
4. Integration Tester validates each service against COBOL behavior contracts
5. Deploy API services alongside running COBOL system with traffic mirroring

## Handoff Protocol

- API Designer outputs endpoint contracts and service boundaries --> consumed by Implementer and Data Mapper
- Data Mapper outputs schema definitions and transformation specs --> consumed by Implementer
- Implementer outputs service code --> consumed by Integration Tester
- Integration Tester can return failing scenarios to Implementer with specific logic gaps
- API Designer updates contracts if Data Mapper discovers interface mismatches
