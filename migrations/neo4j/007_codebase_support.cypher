CREATE INDEX program_codebase IF NOT EXISTS FOR (p:Program) ON (p.codebase);
CREATE INDEX paragraph_codebase IF NOT EXISTS FOR (p:Paragraph) ON (p.codebase);
CREATE INDEX section_codebase IF NOT EXISTS FOR (s:Section) ON (s.codebase);
CREATE INDEX data_item_codebase IF NOT EXISTS FOR (d:DataItem) ON (d.codebase);
CREATE INDEX condition_codebase IF NOT EXISTS FOR (c:Condition) ON (c.codebase);
CREATE INDEX parameter_codebase IF NOT EXISTS FOR (p:Parameter) ON (p.codebase);
CREATE INDEX jcl_job_codebase IF NOT EXISTS FOR (j:JCLJob) ON (j.codebase);
