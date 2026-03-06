// Condition nodes (88-level condition variables)
CREATE INDEX condition_fqn IF NOT EXISTS FOR (c:Condition) ON (c.fqn);
CREATE INDEX condition_programId IF NOT EXISTS FOR (c:Condition) ON (c.programId);

// Parameter nodes (LINKAGE SECTION items)
CREATE INDEX parameter_fqn IF NOT EXISTS FOR (p:Parameter) ON (p.fqn);
CREATE INDEX parameter_programId IF NOT EXISTS FOR (p:Parameter) ON (p.programId);

// DataItem usage index
CREATE INDEX data_item_usage IF NOT EXISTS FOR (d:DataItem) ON (d.usage);

// Program modernization/bridge indexes
CREATE INDEX program_bridge IF NOT EXISTS FOR (p:Program) ON (p.isBridge);
CREATE INDEX program_modernization IF NOT EXISTS FOR (p:Program) ON (p.modernizationScore);

// Copybook risk index
CREATE INDEX copybook_risk IF NOT EXISTS FOR (c:Copybook) ON (c.riskLevel);
