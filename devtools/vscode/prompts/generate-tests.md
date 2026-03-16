---
description: Generate equivalence tests for a COBOL program
mode: agent
tools: ["cobol-graph"]
---

# Generate Equivalence Tests for COBOL Program

Generate a comprehensive test suite for the COBOL program `{programId}` that validates functional equivalence during migration.

## Instructions

Use the cobol-graph MCP tools to understand the program behavior, then generate test cases that cover all paths.

### 1. Get Program Details

- Use `get_program` for program metadata (complexity, domain, LOC)
- Use `get_program_source` to retrieve the original COBOL source code

### 2. Extract Business Logic

- Use `get_paragraph_flow` to map the PERFORM execution graph
- Use `get_program_conditional_logic` to identify all branching paths (IF/EVALUATE/88-level)

### 3. Map Data Items and Parameters

- Use `get_data_items` for all working storage and linkage section items
- Use `get_data_hierarchy` to understand group/elementary relationships
- Use `get_cross_program_data_flow` to identify inputs and outputs passed via LINKAGE

### 4. Identify Error Paths

- Use `get_program_error_handlers` to find all error handling patterns
- Note ABEND codes, error flags, and exception paragraphs

### 5. Analyze Data Flow

- Use `get_data_flow` to trace how data moves through the program
- Identify key transformation points where values are computed or modified

### 6. Check Integration Points for Mocking

- Use `get_program_sql` to catalog SQL operations that need mock data
- Use `get_program_cics` for CICS calls that require stubbing
- Use `get_program_external_interfaces` for external calls that need fakes

### 7. Generate Test Cases

Create tests in the following categories:

- **Happy path** -- Normal execution through the main paragraph flow with valid inputs
- **Boundary conditions** -- PIC field limits, numeric overflow, zero-length strings, max occurs
- **Error handling** -- Each error path identified in step 4, invalid inputs, missing data
- **Validation rules** -- Each conditional branch and 88-level condition
- **Data transformation** -- Verify COMPUTE, MOVE, and arithmetic operations produce correct results
- **Integration** -- SQL result handling, CICS response codes, external call responses

### 8. Create Test Harness

- Define input/output record structures matching COBOL data items
- Provide fixture data for each test scenario
- Include assertions that compare migrated output against expected COBOL behavior
- Document which COBOL paragraph each test case validates
