Extract and document business rules from a COBOL program or business domain. Argument: $ARGUMENTS (program ID or domain name)

## Steps

1. **Resolve target**: If the argument matches a business domain name, call `get_business_domain` to get all programs in that domain. Otherwise, call `get_program` to get single program details.

2. **Get conditional logic**: For each target program, call `get_program_conditional_logic` to extract all IF/EVALUATE/WHEN/PERFORM UNTIL structures. These encode the business rules.

3. **Get condition names**: Call `get_program_conditions` for 88-level condition names -- these are named business states (e.g., `88 VALID-ACCOUNT VALUE 'Y'`).

4. **Analyze paragraph annotations**: Call `get_paragraph_flow` for each program. The paragraph names and annotations from Pass 2 contain semantic descriptions of what each section does.

5. **Map data context**: Call `get_data_items` to understand the data fields referenced in conditions. Call `get_data_hierarchy` to see how fields relate structurally.

6. **Check cross-program rules**: Call `get_cross_program_data_flow` to find rules that span multiple programs (e.g., validation in one program, processing in another). Call `get_shared_data_channels` for shared data pathways.

7. **Identify error/validation rules**: Call `get_program_error_handlers` to find validation and error rules. These often encode critical business constraints.

8. **Synthesize business rules document**: Organize extracted rules by category:
   - **Validation Rules**: Input checks, field validations, 88-level conditions
   - **Calculation Rules**: Arithmetic operations, accumulations, derived fields
   - **Flow Control Rules**: Conditional branching, loop termination conditions
   - **Integration Rules**: When/how external systems are called
   - **Error Handling Rules**: Error conditions and recovery procedures

   For each rule, document:
   - Rule name (derived from paragraph/condition name)
   - Natural language description
   - Source location (program, paragraph)
   - Input fields and expected values
   - Output/side effects
   - Related rules (cross-references)
