package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

func serveIndex(w http.ResponseWriter, r *http.Request) {
	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load ui")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// StaticFiles returns the embedded static filesystem for external serving if needed.
func StaticFiles() fs.FS {
	f, _ := fs.Sub(staticFS, "static")
	return f
}
