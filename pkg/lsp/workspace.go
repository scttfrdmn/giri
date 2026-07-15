package lsp

import (
	"os"
	"strings"
)

// resolveRoot determines the workspace directory from an initialize request,
// preferring rootUri, then the first workspace folder, then the deprecated
// rootPath. It returns "" when the client sent none (the server then analyzes
// the process working directory).
func resolveRoot(p InitializeParams) string {
	if p.RootURI != "" {
		if path := uriToPath(p.RootURI); path != "" {
			return path
		}
	}
	if len(p.WorkspaceFolders) > 0 {
		if path := uriToPath(p.WorkspaceFolders[0].URI); path != "" {
			return path
		}
	}
	return p.RootPath
}

// arenasEnabled reports whether GOEXPERIMENT includes "arenas", which
// go/packages needs to load programs that import the "arena" package.
func arenasEnabled() bool {
	for _, e := range strings.Split(os.Getenv("GOEXPERIMENT"), ",") {
		if strings.TrimSpace(e) == "arenas" {
			return true
		}
	}
	return false
}
