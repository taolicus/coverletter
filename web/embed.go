// Package web holds the browser assets embedded in the server binary.
package web

import "embed"

// Files is the set of browser assets served by the application.
//
//go:embed index.html style.css app.js
var Files embed.FS
