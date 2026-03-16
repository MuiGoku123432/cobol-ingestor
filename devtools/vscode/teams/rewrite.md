---
description: Full system rewrite with 4 specialist agents
mode: agent
tools: ["cobol-graph"]
---

# Full System Rewrite Team

Rewrite the COBOL system from scratch by first documenting all business rules exhaustively, then designing and implementing a modern system purely from those specifications, with continuous validation against the knowledge graph.

## Strategy

A full rewrite decouples from legacy code structure entirely. The key risk is losing business logic buried in decades of COBOL. This team mitigates that risk by using the knowledge graph as the single source of truth: every business rule, data flow, and conditional path is documented before any new code is written, and every implementation is validated against the graph afterward.

## Agent Roles

### 1. Analyst

Extract and document every business rule, data flow, and decision path from the COBOL system.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_program_conditions`, `get_data_flow`, `get_cross_program_data_flow`, `list_business_domains`, `get_business_domain`, `get_program_error_handlers`

**Tasks**:
- Walk every program and document paragraph-level business logic
- Catalog all conditional branches with their conditions and outcomes
- Trace data flows within and across programs to map transformation chains
- Group rules by business domain for organized specification documents
- Document error handling paths and their business implications
- Identify implicit business rules embedded in data validation and MOVE statements
- Produce a comprehensive business rules specification independent of COBOL

### 2. Architect

Design the target system architecture from business specifications alone.

**MCP Tools**: `get_call_chain`, `get_impact_analysis`, `list_business_domains`, `get_shared_data_channels`, `get_migration_sequence`, `list_modernization_candidates`

**Tasks**:
- Define bounded contexts from business domain classifications
- Design service topology based on call chains and impact analysis
- Identify shared data channels that become integration points
- Use modernization candidates to prioritize high-value components
- Plan migration sequence to minimize risk and maximize early value
- Produce architecture decision records and component specifications
- Design the target data model independent of COBOL record structures

### 3. Developer

Implement the target system from architecture specs and business rules documentation.

**MCP Tools**: `get_type_mappings`, `get_data_hierarchy`, `get_copybook_structure`, `get_program_sql`, `get_program_cics`

**Tasks**:
- Implement components following the Architect's design specifications
- Use type mappings as reference for data conversion accuracy
- Translate COBOL data hierarchies into modern domain models
- Convert embedded SQL patterns to modern data access layers
- Replace CICS transaction patterns with modern equivalents
- Reference copybook structures for shared data model fidelity
- Write code from the business rules spec, not from COBOL source

### 4. Validator

Cross-reference every implementation against the knowledge graph to find gaps.

**MCP Tools**: `get_dashboard_stats`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_validation_report`, `get_dead_code_summary`, `get_effort_estimates`

**Tasks**:
- Compare implemented logic against every paragraph flow in the graph
- Verify all conditional branches have corresponding implementations
- Run validation reports to detect missing coverage
- Use dashboard stats to track overall migration completeness
- Exclude dead code paths (confirmed by dead code summary) from gap analysis
- Produce effort estimates for remaining work
- Flag specific gaps with graph references for the Developer to address

## Workflow

1. **Analyst** documents all business rules, data flows, and decision paths into a comprehensive specification
2. **Architect** designs the target system from the specification alone, with no direct COBOL reference
3. **Developer** implements the target system from the architecture design and business rules spec
4. **Validator** cross-references every implementation against the knowledge graph and reports gaps
5. Iterate: gaps found by Validator feed back to Analyst for clarification, then to Developer for implementation
6. Continue until the validation report shows complete coverage

## Handoff Protocol

- Analyst outputs business rules specifications consumed by Architect and Developer
- Architect outputs design documents and component specs consumed by Developer
- Validator reports gaps with specific graph references back to Analyst and Developer
- Each iteration narrows the gap between implementation and the knowledge graph
- Dead code identified by Validator is excluded from coverage requirements
