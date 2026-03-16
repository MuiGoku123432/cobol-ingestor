CREATE INDEX bw_file_path IF NOT EXISTS FOR (n:BWFile) ON (n.path);
CREATE INDEX bw_entity_mergeId IF NOT EXISTS FOR (n:BWEntity) ON (n.mergeId);
CREATE INDEX bw_entity_type IF NOT EXISTS FOR (n:BWEntity) ON (n.entityType);
CREATE INDEX bw_entity_name IF NOT EXISTS FOR (n:BWEntity) ON (n.name);
