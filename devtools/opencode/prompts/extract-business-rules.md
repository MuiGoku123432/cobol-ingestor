# Extract Business Rules

Extract and organize all business rules from a COBOL program or business domain into a structured document.

## Input

- **Program ID or Domain Name**: Either a specific COBOL program identifier or a business domain name to analyze

## Agent Instructions

Use the cobol-graph MCP server tools to identify and extract every business rule, then organize them into a categorized reference document.

### Step 1: Resolve Target

If the input is a program ID, call `get_program` to confirm it exists and note its domain. If the input is a domain name, call `list_business_domains` to verify it, then call `search_programs` filtered by that domain to get all member programs.

### Step 2: Get Conditional Logic

For each target program, call `get_program_conditional_logic` to extract all IF/EVALUATE/WHEN statements. These are the primary carriers of business rules. Record the condition expression, the paragraph where it appears, and both the true and false action branches.

### Step 3: Get 88-Level Conditions

Call `get_data_items` and filter for level-88 condition names. These encode named business states and valid value sets (e.g., `88 VALID-ACCOUNT-TYPE VALUE 'C' 'S' 'M'`). Map each 88-level item to its parent data item and the paragraphs that test it.

### Step 4: Analyze Paragraph Flow and Annotations

Call `get_paragraph_flow` to understand the execution sequence. Call `get_program` to retrieve any paragraph-level annotations that describe business purpose. Paragraphs with names like VALIDATE-, CALC-, CHECK-, or EDIT- often contain concentrated business logic.

### Step 5: Map Data Context

Call `get_data_items` to get field definitions with PIC clauses. Understanding field types and sizes provides context for numeric precision rules, string formatting rules, and valid ranges.

### Step 6: Check Cross-Program Rules

Call `get_cross_program_data_flow` to find rules that span program boundaries, such as validation in one program that depends on values set by another. Call `get_copybook_usage` to identify shared data definitions that encode common business constants or record layouts.

### Step 7: Identify Error and Validation Rules

Call `get_program_error_handlers` to find error handling logic. Error conditions often encode important business constraints (e.g., "account balance must not go negative"). Map each error condition to the business rule it enforces.

### Step 8: Synthesize Business Rules Document

Organize all extracted rules into the following categories:

- **Validation Rules** -- Input validation, field-level edits, format checks, required field checks, cross-field validation
- **Calculation Rules** -- Arithmetic formulas, rounding rules, accumulation logic, rate calculations, pro-ration
- **Flow Control Rules** -- Conditions that determine which processing paths execute, routing logic, skip conditions
- **Integration Rules** -- Rules governing interactions with external systems, file processing sequences, transaction commit/rollback conditions
- **Error Handling Rules** -- Error detection conditions, recovery actions, return code assignments, abend triggers

For each rule, document:

- A plain-language description of what the rule enforces
- The source paragraph and program where it was found
- The COBOL condition expression
- Any related 88-level condition names
- Data items involved with their types and valid ranges
