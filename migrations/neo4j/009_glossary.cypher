CREATE CONSTRAINT glossary_term_key IF NOT EXISTS FOR (g:GlossaryTerm) REQUIRE (g.codebase, g.termLower) IS UNIQUE;
CREATE INDEX glossary_term_kind IF NOT EXISTS FOR (g:GlossaryTerm) ON (g.kind);
CREATE FULLTEXT INDEX glossary_fulltext IF NOT EXISTS FOR (n:GlossaryTerm) ON EACH [n.term, n.definition, n.aliases];
