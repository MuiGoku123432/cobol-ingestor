# Strangler Fig Migration Team

Incrementally replace COBOL functionality by wrapping the legacy system behind a facade and migrating one capability at a time.

## Strategy

The strangler fig pattern allows gradual migration with zero downtime. A routing facade intercepts calls to COBOL programs and redirects them to modern implementations as they become available. Start with leaf-node programs (no downstream calls) and work inward.

## Agent Roles

### Facade Architect

Design the routing/facade layer between callers and COBOL programs.

**MCP Tools**: `get_call_chain`, `get_impact_analysis`, `list_bridge_programs`, `get_program_external_interfaces`, `get_program_parameters`

**Responsibilities**:
- Analyze the call graph to identify all entry points into the COBOL system
- Map LINKAGE SECTION interfaces to API contracts for the facade
- Design routing rules that can toggle between legacy and modern per-program
- Identify bridge programs that serve as natural facade attachment points
- Produce interface contracts for each program to be migrated

### Translator

Convert individual COBOL programs to modern implementations behind the facade.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_data_items`, `get_type_mappings`, `get_program_conditional_logic`, `get_program_sql`, `get_program_cics`

**Responsibilities**:
- Retrieve program source and understand paragraph execution flow
- Map COBOL data types to modern equivalents using type mappings
- Convert each paragraph to a method/function preserving exact logic
- Handle SQL and CICS integration points with modern patterns
- Ensure the migrated program matches the facade's interface contract

### Test Engineer

Create equivalence tests ensuring migrated programs match COBOL behavior.

**MCP Tools**: `get_program_parameters`, `get_program_conditions`, `get_program_conditional_logic`, `get_data_flow`, `get_program_error_handlers`

**Responsibilities**:
- Generate test cases from LINKAGE parameters (input/output contracts)
- Create boundary tests from EVALUATE/WHEN branches
- Test error paths from error handler conditions
- Verify data flow outputs match expected COBOL results
- Build regression suite that runs against both legacy and modern

### Reviewer

Validate no business logic was lost during migration.

**MCP Tools**: `get_paragraph_flow`, `get_program_conditional_logic`, `get_data_flow`, `get_cross_program_data_flow`, `get_validation_report`

**Responsibilities**:
- Compare migrated code against the graph's paragraph flow
- Verify every conditional logic branch has a corresponding implementation
- Check cross-program data flows are preserved
- Run validation report to confirm graph completeness
- Flag missing logic paths and send back for rework

## Workflow

1. Facade Architect analyzes entry points and produces routing design + interface contracts
2. Translator migrates programs starting with leaf nodes (use `get_migration_sequence` for order)
3. Test Engineer creates equivalence tests for each migrated program
4. Reviewer validates completeness against the knowledge graph
5. Facade routes traffic to migrated programs; legacy handles the rest
6. Repeat steps 2-5 for next program in migration sequence

## Handoff Protocol

- Facade Architect outputs interface contracts --> consumed by Translator
- Translator outputs migrated code --> consumed by Test Engineer
- Reviewer can return programs to Translator with specific gap findings
- Each iteration adds one more program behind the facade
