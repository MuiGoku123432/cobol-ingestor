CREATE CONSTRAINT program_id IF NOT EXISTS FOR (p:Program) REQUIRE p.programId IS UNIQUE;
CREATE CONSTRAINT copybook_name IF NOT EXISTS FOR (c:Copybook) REQUIRE c.name IS UNIQUE;
CREATE CONSTRAINT data_item_fqn IF NOT EXISTS FOR (d:DataItem) REQUIRE d.fqn IS UNIQUE;
CREATE INDEX program_file IF NOT EXISTS FOR (p:Program) ON (p.filePath);
CREATE INDEX data_item_level IF NOT EXISTS FOR (d:DataItem) ON (d.level);
CREATE INDEX paragraph_name IF NOT EXISTS FOR (p:Paragraph) ON (p.name);
