package docs

import "embed"

// FS holds markdown docs served at /docs/ and referenced in invite prompts.
//
//go:embed *.md
var FS embed.FS
