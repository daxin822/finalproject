package orchestration

import "embed"

//go:embed templates/*.tmpl
var templateFS embed.FS
