# Event-Driven Decomposition Team

Decompose COBOL batch and online processing into event-driven microservices. CICS transactions become event producers, batch jobs become scheduled event streams, and downstream programs become event consumers with saga-based coordination replacing tight CALL coupling.

## Strategy

Identify events implicit in the COBOL system -- CICS transactions, batch job steps, file I/O operations, and database writes -- and make them explicit as domain events. Each COBOL program becomes one or more microservices that either produce or consume events. Cross-program CALL chains are replaced by event-driven choreography, with sagas managing distributed transactions that were previously handled by COBOL's single-threaded execution model.

## Agent Roles

### Event Modeler

Discover and catalog all implicit events in the COBOL system.

**MCP Tools**: `get_program_cics`, `get_program_external_interfaces`, `get_program_sql`, `list_jcl_jobs`, `get_jcl_job`, `get_dataset_usage`, `get_file_accessors`, `list_business_domains`

**Responsibilities**:
- Catalog CICS transactions as candidate command/event pairs
- Map JCL job steps to batch event sequences with dependencies
- Identify file and dataset operations as data change events
- Detect SQL write operations as state change events
- Group events by business domain into event streams
- Produce an event catalog with schemas, producers, consumers, and ordering constraints

### Producer Builder

Build event-producing microservices from COBOL program write paths.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_data_items`, `get_type_mappings`, `get_program_sql`, `get_program_cics`

**Responsibilities**:
- Extract write-path logic from programs that generate state changes
- Convert CICS SEND/RECEIVE patterns to event publish operations
- Transform SQL INSERT/UPDATE operations into event emissions
- Map COBOL data items to event payload schemas using type mappings
- Ensure event ordering matches the original paragraph execution flow
- Build idempotent producers that guarantee at-least-once delivery

### Consumer Builder

Build event-consuming microservices from COBOL program read and processing paths.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_data_items`, `get_type_mappings`, `get_program_error_handlers`

**Responsibilities**:
- Extract processing logic from programs that react to inputs or state changes
- Convert conditional logic branches to event handler routing
- Implement error handlers as dead-letter queue processors
- Map input data items to event consumption schemas
- Build idempotent consumers that handle duplicate event delivery
- Ensure processing order matches COBOL paragraph flow semantics

### Saga Coordinator

Design distributed transaction patterns replacing COBOL CALL chains.

**MCP Tools**: `get_call_chain`, `get_cross_program_data_flow`, `get_shared_data_channels`, `get_impact_analysis`, `get_program_table_access`, `get_table_usage`

**Responsibilities**:
- Map CALL chains to saga step sequences with compensating actions
- Identify shared data channels that require eventual consistency patterns
- Design compensation logic for each step that can fail
- Analyze table access patterns to determine isolation requirements
- Define saga timeouts and retry policies from impact analysis
- Produce saga definitions with step ordering, compensation, and failure handling

## Workflow

1. Event Modeler catalogs all implicit events from CICS, JCL, file I/O, and SQL operations
2. Producer Builder and Consumer Builder work in parallel on their respective microservices
3. Saga Coordinator designs distributed transaction patterns for cross-service workflows
4. Integration testing validates event flow matches original COBOL processing order
5. Deploy with event sourcing, enabling replay and audit of all state transitions

## Handoff Protocol

- Event Modeler outputs event catalog with schemas --> consumed by Producer Builder and Consumer Builder
- Producer Builder outputs event producer services --> Saga Coordinator uses for step definitions
- Consumer Builder outputs event consumer services --> Saga Coordinator uses for compensation design
- Saga Coordinator outputs saga definitions --> consumed by Producer Builder and Consumer Builder for transaction boundary implementation
- Event Modeler revises catalog if builders discover undocumented events during implementation
