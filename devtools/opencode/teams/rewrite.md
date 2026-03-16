# Full System Rewrite Team

Document everything first, then rebuild the system from specification. The knowledge graph serves as the single source of truth for what the COBOL system does, enabling a clean-room rewrite that is faithful to business logic without carrying forward legacy implementation patterns.

## Strategy

Exhaustively document the existing COBOL system's behavior using the knowledge graph, then design and build a modern replacement from those specifications alone. The rewrite team never references COBOL source directly during implementation -- only the documented specifications. This produces a modern system that preserves business logic while adopting current architectural patterns.

## Agent Roles

### Analyst

Extract and document every behavior, rule, and data flow from the COBOL system.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_program_conditions`, `get_data_flow`, `get_cross_program_data_flow`, `list_business_domains`, `get_business_domain`, `get_program_error_handlers`

**Responsibilities**:
- Document each program's purpose, inputs, outputs, and business rules
- Trace paragraph execution flows and capture all conditional logic branches
- Map data flows within and across programs to identify business processes
- Catalog error handling behavior and edge cases
- Organize findings by business domain for architectural clarity
- Produce a complete behavioral specification for every program

### Architect

Design the modern system from the Analyst's specifications.

**MCP Tools**: `get_call_chain`, `get_impact_analysis`, `list_business_domains`, `get_shared_data_channels`, `get_migration_sequence`, `list_modernization_candidates`

**Responsibilities**:
- Design system architecture from behavioral specifications (not from COBOL structure)
- Define bounded contexts using business domain groupings
- Plan migration sequence based on dependency analysis and modernization candidates
- Identify shared data channels that become integration points or shared services
- Produce technical design documents with component interfaces and data models
- Ensure the architecture addresses all behaviors documented by the Analyst

### Developer

Implement the modern system from the Architect's design.

**MCP Tools**: `get_type_mappings`, `get_data_hierarchy`, `get_copybook_structure`, `get_program_sql`, `get_program_cics`

**Responsibilities**:
- Build components following the Architect's design specifications
- Use type mappings to ensure data fidelity between legacy and modern representations
- Translate SQL access patterns to modern data layer implementations
- Replace CICS transaction patterns with modern equivalents
- Reference copybook structures only for data format accuracy, not implementation style
- Write unit tests for each component against the Analyst's behavioral specs

### Validator

Confirm the rewritten system covers all documented behavior.

**MCP Tools**: `get_dashboard_stats`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_validation_report`, `get_dead_code_summary`, `get_effort_estimates`

**Responsibilities**:
- Cross-reference the rewritten system against the Analyst's behavioral specifications
- Verify every conditional logic path has a corresponding implementation
- Use dashboard stats to confirm full program coverage
- Review dead code summary to exclude behaviors that are no longer needed
- Produce effort estimates for remaining gaps
- Run validation reports to ensure the knowledge graph is fully accounted for

## Workflow

1. Analyst documents the entire COBOL system's behavior from the knowledge graph
2. Architect designs the modern system from specifications only (no COBOL reference)
3. Developer implements the design using type mappings and data structures for accuracy
4. Validator cross-references the implementation against documented behaviors
5. Iterate on gaps -- Analyst clarifies, Architect adjusts, Developer fixes, Validator re-checks

## Handoff Protocol

- Analyst outputs behavioral specifications --> consumed by Architect and Validator
- Architect outputs technical design documents --> consumed by Developer
- Developer outputs implemented components --> consumed by Validator
- Validator returns coverage gaps to Analyst for clarification or Developer for fixes
- Architect revises design if Validator identifies structural gaps
