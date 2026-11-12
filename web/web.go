// Package web embeds the administrator HTML templates and styles into the binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.html static/css/*.css static/js/*.js static/assets/*
var embedded embed.FS

// Assets is the read-only filesystem served by the Warden Web UI.
var Assets fs.FS = embedded
