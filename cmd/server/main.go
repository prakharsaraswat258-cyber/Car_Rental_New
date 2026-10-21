// Command server is the same app as cmd/saarthi, at the path Vercel's Go
// preset looks for (cmd/server/main.go). Locally, use cmd/saarthi.
package main

import (
	"os"

	"saarthi/internal/app"
)

func main() { os.Exit(app.Main(os.Args[1:])) }
