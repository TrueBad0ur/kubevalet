package web

import "embed"

// FS holds the compiled frontend assets built by Vite.
// Run `make web-build` or let the Dockerfile do it before `go build`.
//
//go:embed dist
var FS embed.FS
