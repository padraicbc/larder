// Package webui embeds the built SvelteKit app so the server (and the
// Android library) ship as a single binary. Run `make build` to fill dist/.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built app, and false if dist/ only holds the placeholder.
func FS() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "200.html"); err != nil {
		return nil, false
	}
	return sub, true
}
