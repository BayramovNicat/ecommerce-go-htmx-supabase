package web

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed templates/**/*.html
var embeddedFS embed.FS

var FS fs.FS

func init() {
	// Use filesystem in development, embedded in production
	if os.Getenv("ENV") == "production" {
		FS = embeddedFS
	} else {
		FS = os.DirFS("web")
	}
}
