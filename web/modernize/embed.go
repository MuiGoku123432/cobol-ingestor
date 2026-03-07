package modernize

import "embed"

//go:embed index.html app.js
var StaticFS embed.FS
