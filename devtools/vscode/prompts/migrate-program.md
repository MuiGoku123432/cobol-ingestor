---
description: Migrate a COBOL program to a modern language
mode: agent
tools: ["cobol-graph"]
---

# Migrate COBOL Program

Migrate the COBOL program `{programId}` to a modern implementation.

## Instructions

Use the cobol-graph MCP tools to gather complete information about this program, then generate a modern equivalent.

### 1. Understand the Program

- Use `get_program` to get program metadata (complexity, domain, LOC)
- Use `get_program_source` to retrieve the original COBOL source

### 2. Map the Call Chain

- Use `get_call_chain` with direction "both" to find callers and callees
- Use `get_impact_analysis` to assess change blast radius

### 3. Analyze Data Structures

- Use `get_data_items` for working storage and linkage items
- Use `get_data_hierarchy` for group/elementary relationships
- Use `get_type_mappings` for target language type conversions

### 4. Understand Program Flow

- Use `get_paragraph_flow` for the PERFORM execution graph
- Use `get_program_conditional_logic` for all branching logic

### 5. Map Integration Points

- Use `get_program_sql` for embedded SQL
- Use `get_program_cics` for CICS transactions
- Use `get_program_external_interfaces` for external calls
- Use `get_program_error_handlers` for error patterns

### 6. Analyze Dependencies

- Use `get_copybook_usage` and `get_copybook_structure` for copybook data
- Use `get_data_flow` and `get_cross_program_data_flow` for data movement

### 7. Generate Migration

- Map paragraphs to methods/functions
- Convert data items using type mappings
- Replace PERFORM logic with method calls
- Convert SQL/CICS to modern equivalents
- Preserve all business logic and error handling
- Add comments referencing original COBOL paragraph names
