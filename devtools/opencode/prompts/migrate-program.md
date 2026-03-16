# Migrate COBOL Program

Migrate a COBOL program to a modern language implementation.

## Input

- **Program ID**: The COBOL program identifier to migrate

## Agent Instructions

Use the cobol-graph MCP server tools to gather complete information about the target program, then generate a modern equivalent.

### Step 1: Understand the Program

Call `get_program` with the program ID to get metadata (complexity, domain, LOC). Call `get_program_source` to retrieve the original COBOL source code.

### Step 2: Map the Call Chain

Call `get_call_chain` with direction "both" to find all callers and callees. Call `get_impact_analysis` to understand the blast radius of changes.

### Step 3: Analyze Data Structures

Call `get_data_items` for working storage and linkage section items. Call `get_data_hierarchy` for group/elementary relationships. Call `get_type_mappings` for target language type conversions.

### Step 4: Understand Program Flow

Call `get_paragraph_flow` for the PERFORM execution graph. Call `get_program_conditional_logic` for all IF/EVALUATE/WHEN branching.

### Step 5: Map Integration Points

- Call `get_program_sql` for embedded SQL statements
- Call `get_program_cics` for CICS transaction handling
- Call `get_program_external_interfaces` for external system calls
- Call `get_program_error_handlers` for error handling patterns

### Step 6: Analyze Dependencies

Call `get_copybook_usage` and `get_copybook_structure` for shared data definitions. Call `get_data_flow` and `get_cross_program_data_flow` for data movement patterns.

### Step 7: Generate Migration

Based on all gathered information:

- Map COBOL paragraphs to methods/functions
- Convert data items using type mappings
- Replace PERFORM logic with method calls
- Convert SQL/CICS to modern equivalents
- Preserve all business logic and error handling
- Add comments referencing original COBOL paragraph names

### Step 8: Validate Completeness

Cross-reference generated code against the paragraph flow. Ensure no logic paths were missed and all CALL targets have corresponding service references.
