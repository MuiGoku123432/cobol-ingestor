package prompts

import _ "embed"

//go:embed pass1_structural.tmpl
var Pass1Structural string

//go:embed pass2_deep.tmpl
var Pass2Deep string

//go:embed pass3_crosscutting.tmpl
var Pass3CrossCutting string

//go:embed pass1_jcl.tmpl
var Pass1JCL string

//go:embed pass4_crossprogram.tmpl
var Pass4CrossProgram string

//go:embed pass5_repair.tmpl
var Pass5Repair string

//go:embed extdb_analysis.tmpl
var ExtDBAnalysis string

//go:embed bw_ingest.tmpl
var BWIngest string

//go:embed bw_pass2_synthesis.tmpl
var BWPass2Synthesis string

//go:embed bw_pass3_repair.tmpl
var BWPass3Repair string

//go:embed pass4_commarea.tmpl
var Pass4Commarea string

//go:embed pass4_file_flow.tmpl
var Pass4FileFlow string

//go:embed pass5_dead_verify.tmpl
var Pass5DeadVerify string

//go:embed pass5_domain_merge.tmpl
var Pass5DomainMerge string

//go:embed examples/pass1_example.txt
var Pass1Example string

//go:embed examples/pass2_example.txt
var Pass2Example string
