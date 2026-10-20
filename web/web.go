// Package web embeds Saarthi's templates and static files into the binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var embedded embed.FS

// FS is rooted at web/: it holds templates/… and static/….
var FS fs.FS = embedded
