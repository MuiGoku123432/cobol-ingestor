Analyze dependencies and impact radius for a COBOL program. Argument: $ARGUMENTS (program ID)

## Steps

1. **Get program overview**: Call `get_program` with the program ID for metadata. Note the business domain, complexity score, and line count.

2. **Trace call chains**: Call `get_call_chain` with direction "downstream" to find all programs this one calls. Then call with direction "upstream" to find all callers. Call `list_bridge_programs` to identify if this is a bridge/integration point.

3. **Run impact analysis**: Call `get_impact_analysis` with the program ID. This returns the full blast radius -- all programs, copybooks, and data items affected by changes to this program.

4. **Analyze shared copybooks**: Call `get_copybook_usage` to find all copybooks this program includes. For each copybook, note how many other programs also include it -- high-sharing copybooks are high-risk change points. Call `list_copybook_risks` for risk assessment.

5. **Trace cross-program data flow**: Call `get_cross_program_data_flow` with the program ID to find data shared via files, DB2 tables, or VSAM. Call `get_shared_data_channels` to identify all shared data pathways.

6. **Check database dependencies**: Call `get_program_table_access` to find all DB tables this program reads/writes. Call `get_table_usage` for each table to find other programs sharing those tables.

7. **Map external interfaces**: Call `get_program_external_interfaces` for external system integrations. Call `get_program_jcl` to find JCL jobs that execute this program.

8. **Assess migration risk**: Call `get_effort_estimates` for effort scoring. Call `get_migration_sequence` to understand where this program falls in the recommended migration order.

9. **Generate dependency report**: Summarize findings as:
   - Direct dependencies (calls out)
   - Reverse dependencies (called by)
   - Shared data (copybooks, files, tables)
   - Integration points (CICS, external, JCL)
   - Risk score and recommended migration order position
