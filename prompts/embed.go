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
