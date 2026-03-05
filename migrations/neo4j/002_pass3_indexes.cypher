CREATE INDEX paragraph_programId IF NOT EXISTS FOR (p:Paragraph) ON (p.programId);
CREATE INDEX section_programId IF NOT EXISTS FOR (s:Section) ON (s.programId);
CREATE CONSTRAINT domain_name IF NOT EXISTS FOR (b:BusinessDomain) REQUIRE b.name IS UNIQUE;
CREATE FULLTEXT INDEX search_programs IF NOT EXISTS FOR (p:Program) ON EACH [p.programId, p.filePath];
CREATE FULLTEXT INDEX search_paragraphs IF NOT EXISTS FOR (p:Paragraph) ON EACH [p.name, p.description]
