CREATE INDEX ext_database_name IF NOT EXISTS FOR (n:ExternalDatabase) ON (n.name);
CREATE INDEX ext_db_table_name IF NOT EXISTS FOR (n:ExternalDBTable) ON (n.name);
