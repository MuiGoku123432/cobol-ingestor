// Migration 010: Multi-language node constraints and indexes
// Supports C, PL/SQL, Korn shell, Custom (--all-extensions), and ISAM file nodes

// ── C ──────────────────────────────────────────────────────────────────────
CREATE CONSTRAINT c_program_filepath IF NOT EXISTS
  FOR (n:CProgram) REQUIRE (n.filePath, n.codebase) IS NODE KEY;

CREATE CONSTRAINT c_function_mergeid IF NOT EXISTS
  FOR (n:CFunction) REQUIRE n.mergeId IS UNIQUE;

CREATE CONSTRAINT c_struct_mergeid IF NOT EXISTS
  FOR (n:CStruct) REQUIRE n.mergeId IS UNIQUE;

CREATE CONSTRAINT c_typedef_mergeid IF NOT EXISTS
  FOR (n:CTypedef) REQUIRE n.mergeId IS UNIQUE;

CREATE CONSTRAINT c_header_mergeid IF NOT EXISTS
  FOR (n:CHeader) REQUIRE n.mergeId IS UNIQUE;

CREATE INDEX c_function_name IF NOT EXISTS FOR (n:CFunction) ON (n.name);
CREATE INDEX c_program_codebase IF NOT EXISTS FOR (n:CProgram) ON (n.codebase);

// ── PL/SQL ─────────────────────────────────────────────────────────────────
CREATE CONSTRAINT plsql_package_name IF NOT EXISTS
  FOR (n:PLSQLPackage) REQUIRE n.name IS UNIQUE;

CREATE CONSTRAINT plsql_proc_mergeid IF NOT EXISTS
  FOR (n:PLSQLProcedure) REQUIRE n.mergeId IS UNIQUE;

CREATE CONSTRAINT plsql_func_mergeid IF NOT EXISTS
  FOR (n:PLSQLFunction) REQUIRE n.mergeId IS UNIQUE;

CREATE CONSTRAINT plsql_trigger_mergeid IF NOT EXISTS
  FOR (n:PLSQLTrigger) REQUIRE n.mergeId IS UNIQUE;

CREATE CONSTRAINT plsql_cursor_mergeid IF NOT EXISTS
  FOR (n:PLSQLCursor) REQUIRE n.mergeId IS UNIQUE;

CREATE CONSTRAINT plsql_type_mergeid IF NOT EXISTS
  FOR (n:PLSQLType) REQUIRE n.mergeId IS UNIQUE;

CREATE INDEX plsql_package_codebase IF NOT EXISTS FOR (n:PLSQLPackage) ON (n.codebase);
CREATE INDEX plsql_proc_name IF NOT EXISTS FOR (n:PLSQLProcedure) ON (n.name);

// ── Korn Shell ─────────────────────────────────────────────────────────────
CREATE CONSTRAINT shell_script_filepath IF NOT EXISTS
  FOR (n:ShellScript) REQUIRE (n.filePath, n.codebase) IS NODE KEY;

CREATE CONSTRAINT shell_function_mergeid IF NOT EXISTS
  FOR (n:ShellFunction) REQUIRE n.mergeId IS UNIQUE;

CREATE INDEX shell_script_codebase IF NOT EXISTS FOR (n:ShellScript) ON (n.codebase);

// ── Custom (--all-extensions proprietary formats) ──────────────────────────
CREATE CONSTRAINT custom_entity_mergeid IF NOT EXISTS
  FOR (n:CustomEntity) REQUIRE n.mergeId IS UNIQUE;

CREATE INDEX custom_entity_type IF NOT EXISTS FOR (n:CustomEntity) ON (n.entityType);
CREATE INDEX custom_entity_extension IF NOT EXISTS FOR (n:CustomEntity) ON (n.extension);
CREATE INDEX custom_entity_codebase IF NOT EXISTS FOR (n:CustomEntity) ON (n.codebase);

CREATE FULLTEXT INDEX custom_entity_fulltext IF NOT EXISTS
  FOR (n:CustomEntity) ON EACH [n.name, n.description, n.entityType];

// ── ISAM Files ─────────────────────────────────────────────────────────────
CREATE CONSTRAINT isam_file_name IF NOT EXISTS
  FOR (n:ISAMFile) REQUIRE n.name IS UNIQUE;

CREATE INDEX isam_file_codebase IF NOT EXISTS FOR (n:ISAMFile) ON (n.codebase);
