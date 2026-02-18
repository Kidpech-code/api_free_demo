package main

import "embed"

// WebFS holds the compiled-in web UI files served at the root of the HTTP server.
// All files inside cmd/api/web/ are embedded directly into the binary at build time —
// no separate file copy or volume mount is needed in the container.
//
//go:embed web
var WebFS embed.FS
