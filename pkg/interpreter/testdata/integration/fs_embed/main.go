// fs_embed exercises io/fs and embed package intercepts (#109).
//
// Expected: 0 violations.
package main

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed testdata.txt
var embedded embed.FS

func useEmbedFS() {
	// embed.FS methods (pkgPath = "embed")
	data, _ := embedded.ReadFile("testdata.txt")
	_ = data

	entries, _ := embedded.ReadDir(".")
	_ = entries

	f, _ := embedded.Open("testdata.txt")
	_ = f
}

func useIOFS(fsys fs.FS) {
	// io/fs standalone functions (pkgPath = "io/fs")
	_ = fs.ValidPath("testdata.txt")

	content, _ := fs.ReadFile(fsys, "testdata.txt")
	_ = content

	dirs, _ := fs.ReadDir(fsys, ".")
	_ = dirs

	info, _ := fs.Stat(fsys, "testdata.txt")
	_ = info

	sub, _ := fs.Sub(fsys, ".")
	_ = sub

	_ = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		return nil
	})
}

func main() {
	useEmbedFS()
	fsys := os.DirFS(".")
	useIOFS(fsys)
}
