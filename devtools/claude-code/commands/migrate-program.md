Migrate a COBOL program to a modern language. Argument: $ARGUMENTS (program ID)

## Steps

1. **Understand the program**: Call `get_program` with the program ID to get metadata (lines of code, complexity, business domain). Then call `get_program_source` to retrieve the original COBOL source.

2. **Map the call chain**: Call `get_call_chain` with the program ID and direction "both" to understand upstream callers and downstream callees. Call `get_impact_analysis` to assess blast radius.

3. **Analyze data structures**: Call `get_data_items` for the program's working storage and linkage section items. Call `get_data_hierarchy` to understand group/elementary relationships. Call `get_type_mappings` to get suggested target language type conversions.

4. **Understand program flow**: Call `get_paragraph_flow` to get the PERFORM graph showing paragraph execution order. Call `get_program_conditions` and `get_program_conditional_logic` to map branching logic.

5. **Map integration points**:
   - Call `get_program_sql` for embedded SQL statements
   - Call `get_program_cics` for CICS transaction handling
   - Call `get_program_external_interfaces` for external system calls
   - Call `get_program_error_handlers` for error handling patterns

6. **Check copybook dependencies**: Call `get_copybook_usage` to find included copybooks. Call `get_copybook_structure` for each copybook to understand shared data definitions.

7. **Analyze data flow**: Call `get_data_flow` for the program to understand MOVES_TO relationships. Call `get_cross_program_data_flow` if the program shares data with other programs via files or DB.

8. **Generate migration scaffold**: Based on all gathered information:
   - Map COBOL paragraphs to methods/functions
   - Convert data items using type mappings
   - Replace PERFORM logic with method calls
   - Convert SQL/CICS to modern equivalents
   - Preserve all business logic and error handling
   - Add comments referencing original COBOL paragraph names

9. **Validate completeness**: Cross-reference the generated code against the paragraph flow to ensure no logic paths were missed. Check that all CALL targets have corresponding service references.
