# Generate Test Cases for COBOL Program

Generate comprehensive test cases for a COBOL program based on its extracted structure, business logic, and integration points.

## Input

- **Program ID**: The COBOL program identifier to generate tests for

## Agent Instructions

Use the cobol-graph MCP server tools to understand the program thoroughly, then generate a complete test suite.

### Step 1: Get Program Details and Source

Call `get_program` with the program ID to retrieve metadata. Call `get_program_source` to get the original COBOL source code for reference.

### Step 2: Extract Business Logic

Call `get_paragraph_flow` to map the PERFORM execution graph and identify all logic paths. Call `get_program_conditional_logic` to extract every IF/EVALUATE/WHEN branch, which defines the decision points that tests must cover.

### Step 3: Map Data Items and Parameters

Call `get_data_items` to list all working storage and linkage section items with their PIC clauses and initial values. Call `get_data_hierarchy` to understand group/elementary relationships. Pay special attention to linkage section items as these are the program's input/output interface.

### Step 4: Identify Error Paths

Call `get_program_error_handlers` to find all error handling paragraphs and their trigger conditions. Note return codes, abend handling, and error flags that tests should exercise.

### Step 5: Analyze Data Flow

Call `get_data_flow` to trace how data moves through the program from input to output. Call `get_cross_program_data_flow` to understand linkage parameter contracts with callers and callees.

### Step 6: Check Integration Points for Mocking

- Call `get_program_sql` to identify embedded SQL operations that need database mocks
- Call `get_program_cics` to find CICS transactions requiring transaction mocks
- Call `get_program_external_interfaces` to locate external calls needing service stubs

### Step 7: Generate Test Cases

Produce test cases organized into the following categories:

- **Happy Path Tests** -- Standard execution through the main logic flow with valid inputs, verifying expected outputs and return codes
- **Boundary Tests** -- Edge cases for numeric fields (zero, max PIC value, negative), string fields (empty, max length, special characters), and date fields (leap year, end of month, century boundary)
- **Error Handling Tests** -- Tests that trigger each identified error path, verifying correct error codes, messages, and recovery behavior
- **Validation Tests** -- Tests for every conditional branch, ensuring both true and false paths execute correctly with appropriate data
- **Integration Tests** -- Tests for SQL, CICS, and external interface interactions using mocks that verify correct call sequences and parameter passing

### Step 8: Create Test Harness

Define a test harness structure that includes:

- Mock definitions for each external dependency (DB2 tables, CICS commands, external calls)
- Setup routines that initialize working storage to known states
- Assertion helpers for comparing actual vs expected field values
- Teardown routines for cleanup
- A test data matrix mapping each test case to its input values, expected outputs, and the logic paths it exercises
