Generate equivalence tests for a COBOL program's business logic. Argument: $ARGUMENTS (program ID)

## Steps

1. **Get program details**: Call `get_program` with the program ID. Call `get_program_source` to retrieve the original COBOL source for reference.

2. **Extract business logic**: Call `get_paragraph_flow` to understand the execution flow. Call `get_program_conditional_logic` to extract all IF/EVALUATE/WHEN conditions -- these define the program's decision points.

3. **Map data items**: Call `get_data_items` for all working storage and linkage items. Call `get_data_hierarchy` to understand group structures. Call `get_type_mappings` for target language type equivalents.

4. **Understand parameters**: Call `get_program_parameters` to get LINKAGE SECTION parameters -- these are the program's input/output contract. Call `get_program_conditions` for 88-level condition names.

5. **Identify error paths**: Call `get_program_error_handlers` to find error handling paragraphs and their trigger conditions.

6. **Analyze data flow**: Call `get_data_flow` to understand how data moves through the program -- which fields feed into conditions and which are modified as outputs.

7. **Check integration points**: Call `get_program_sql` for SQL operations that need mocking. Call `get_program_cics` for CICS operations. Call `get_program_external_interfaces` for external calls.

8. **Generate test cases**: For each paragraph with business logic:
   - Create happy-path tests based on normal flow conditions
   - Create boundary tests from EVALUATE/WHEN branches
   - Create error-path tests from error handler triggers
   - Create data validation tests from 88-level conditions
   - Mock SQL/CICS/external calls with expected responses
   - Verify output data items match expected values

9. **Create test harness**: Generate a test file that:
   - Sets up test data matching COBOL data item structures
   - Provides mock implementations for external dependencies
   - Runs each test case and compares against expected COBOL behavior
   - Reports coverage of paragraphs and condition branches
