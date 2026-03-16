---
description: Extract and document business rules from a COBOL program or domain
mode: agent
tools: ["cobol-graph"]
---

# Extract Business Rules

Extract and document all business rules from the COBOL program or domain `{target}`.

## Instructions

Use the cobol-graph MCP tools to systematically identify every business rule, then produce an organized rules document.

### 1. Resolve Target

- If `{target}` is a program ID, use `get_program` to retrieve its metadata
- If `{target}` is a business domain, use `list_business_domains` and `get_domain_programs` to find all programs in that domain

### 2. Get Conditional Logic

- Use `get_program_conditional_logic` for each program to extract all IF, EVALUATE, and conditional statements
- Pay special attention to nested conditions and compound boolean expressions

### 3. Get 88-Level Conditions

- Use `get_data_items` and filter for 88-level items, which represent named business conditions
- Map each 88-level to its parent data item and valid values

### 4. Analyze Paragraph Annotations

- Use `get_paragraph_flow` to get paragraph names and annotations
- Paragraph names in COBOL often encode business intent (e.g., VALIDATE-ACCOUNT, CALC-INTEREST)

### 5. Map Data Context

- Use `get_data_items` for working storage items that hold business state
- Use `get_data_hierarchy` to understand how business fields relate to record structures
- Use `get_data_flow` to trace how business values are computed and propagated

### 6. Check Cross-Program Rules

- Use `get_call_chain` to find programs that participate in the same business process
- Use `get_cross_program_data_flow` to trace business data across program boundaries
- Use `get_copybook_structure` for shared data layouts that encode business constraints

### 7. Identify Error and Validation Rules

- Use `get_program_error_handlers` for error conditions that represent business violations
- Use `get_program_sql` for database constraints enforced in SQL (WHERE clauses, CHECK constraints)

### 8. Synthesize Business Rules Document

Organize the extracted rules into a structured document:

- **Validation rules** -- Input checks, field constraints, 88-level conditions
- **Calculation rules** -- Formulas, rate tables, rounding behavior
- **Flow rules** -- Conditional routing, process sequencing, paragraph branching
- **Domain rules** -- Business-specific logic (eligibility, pricing, classification)
- **Integration rules** -- Cross-program contracts, shared data constraints
- **Error rules** -- Business exception conditions and their handling

For each rule, include:
- A plain-language description
- The source paragraph and program where it was found
- The COBOL condition or computation that implements it
- Any related 88-level names or data items
