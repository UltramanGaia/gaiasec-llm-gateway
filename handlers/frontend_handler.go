package handlers

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// FrontendHandler serves the embedded frontend SPA.
// Pass nil to disable (serve a plain "ok" response instead).
type FrontendHandler struct {
	fs http.FileSystem
}

func NewFrontendHandler(embedded embed.FS, subdir string) (*FrontendHandler, error) {
	sub, err := fs.Sub(embedded, subdir)
	if err != nil {
		return nil, err
	}
	return &FrontendHandler{fs: http.FS(sub)}, nil
}

func (h *FrontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.fs == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("ok"))
		return
	}

	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index.html"
	}

	// Try to open the exact file; fall back to index.html for SPA routing.
	f, err := h.fs.Open(path)
	if err != nil {
		r2 := *r
		r2.URL.Path = "/index.html"
		http.FileServer(h.fs).ServeHTTP(w, &r2)
		return
	}
	f.Close()

	// Prevent directory listings.
	if strings.HasSuffix(path, "/") {
		r2 := *r
		r2.URL.Path = "/index.html"
		http.FileServer(h.fs).ServeHTTP(w, &r2)
		return
	}

	http.FileServer(h.fs).ServeHTTP(w, r)
}
