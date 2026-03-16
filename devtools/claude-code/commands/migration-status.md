Report on the overall migration status and planning for the COBOL codebase. Argument: $ARGUMENTS (optional: specific domain or "all")

## Steps

1. **Get dashboard overview**: Call `get_dashboard_stats` to retrieve high-level statistics: total programs, copybooks, relationships, domains, and complexity distribution.

2. **List business domains**: Call `list_business_domains` to see how the codebase is organized by functional area. Note program counts and complexity per domain.

3. **Get modernization candidates**: Call `list_modernization_candidates` to find programs flagged for modernization with their scores and reasons.

4. **Get risk assessment**: Call `list_risk_programs` to identify high-risk programs (high complexity, many dependencies, critical business functions).

5. **Get volume estimates**: Call `list_volume_estimates` for sizing information -- lines of code, number of data items, SQL statements per program.

6. **Get effort estimates**: Call `get_effort_estimates` for estimated migration effort per program/domain.

7. **Get migration sequence**: Call `get_migration_sequence` to retrieve the recommended order of migration based on dependency analysis and risk scoring.

8. **Check dead code**: Call `get_dead_code_summary` to identify dead paragraphs and unreachable code that can be excluded from migration scope.

9. **Run validation**: Call `get_validation_report` to check graph completeness -- missing relationships, unresolved calls, programs without domain assignments.

10. **Generate status report**: Compile findings into a migration status report:
    - **Scope**: Total programs, LOC, domains, complexity breakdown
    - **Readiness**: Validation status, graph completeness, known gaps
    - **Risk Profile**: High/medium/low risk distribution, critical dependencies
    - **Dead Code**: Paragraphs/programs that can be excluded
    - **Recommended Sequence**: First wave candidates (low risk, few dependencies)
    - **Effort Summary**: Estimated effort by domain and total
    - **Blockers**: Unresolved calls, missing copybooks, validation failures
