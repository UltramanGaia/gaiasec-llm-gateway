package main

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

type hybridFS struct {
	localDir   string
	embeddedFS fs.FS
	useLocal   bool
}

func newHybridFS(localDir string, embeddedFS fs.FS) *hybridFS {
	_, err := os.Stat(localDir)
	useLocal := err == nil
	return &hybridFS{
		localDir:   localDir,
		embeddedFS: embeddedFS,
		useLocal:   useLocal,
	}
}

func (h *hybridFS) Open(name string) (fs.File, error) {
	if h.useLocal {
		file, err := os.Open(h.localDir + "/" + name)
		if err == nil {
			return file, nil
		}
	}
	return h.embeddedFS.Open(name)
}

func getFrontendFS() *hybridFS {
	subFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		panic(err)
	}
	return newHybridFS("./frontend/dist", subFS)
}

func getAssetsFS() *hybridFS {
	subFS, err := fs.Sub(frontendFS, "frontend/dist/assets")
	if err != nil {
		panic(err)
	}
	return newHybridFS("./frontend/dist/assets", subFS)
}
