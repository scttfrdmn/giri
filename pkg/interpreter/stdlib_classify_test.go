package interpreter

import "testing"

// TestIsStdlibOrKnownExternalPkg checks that only real GOROOT packages and
// golang.org/x modules are treated as intercept-only (#249).
func TestIsStdlibOrKnownExternalPkg(t *testing.T) {
	for path, want := range map[string]bool{
		"strings":                    true,
		"net/http":                   true,
		"math/bits":                  true,
		"golang.org/x/text/language": true,
		"probe":                      false, // dotless module path
		"command-line-arguments":     false, // file-loaded main package
		"gen/kernels":                false,
		"github.com/scttfrdmn/giri":  false,
		"":                           false,
	} {
		if got := isStdlibOrKnownExternalPkg(path); got != want {
			t.Errorf("isStdlibOrKnownExternalPkg(%q) = %v, want %v", path, got, want)
		}
	}
}
