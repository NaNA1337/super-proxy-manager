package embedded

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// GetDistFS returns the sub filesystem rooted at "dist"
func GetDistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
