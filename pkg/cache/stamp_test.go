package cache

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestBuildInfoFingerprint(t *testing.T) {
	vcs := func(rev, modified string) []debug.BuildSetting {
		return []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: modified}}
	}
	main := func(v string) debug.Module { return debug.Module{Path: "github.com/scttfrdmn/giri", Version: v} }
	cases := []struct {
		name string
		bi   debug.BuildInfo
		want string // "" = not usable, fall back to the executable hash
	}{
		{"clean checkout", debug.BuildInfo{Main: main("(devel)"), Settings: vcs("abc123", "false")}, "vcs:abc123"},
		{"dirty checkout", debug.BuildInfo{Main: main("v0.97.1-0.2026-abc123+dirty"), Settings: vcs("abc123", "true")}, ""},
		{"go install @tag", debug.BuildInfo{Main: main("v0.97.0")}, "mod:github.com/scttfrdmn/giri@v0.97.0"},
		{"go install @commit", debug.BuildInfo{Main: main("v0.97.1-0.20261029-0e7b1ae5bb96")}, "mod:github.com/scttfrdmn/giri@v0.97.1-0.20261029-0e7b1ae5bb96"},
		{"go run / go test", debug.BuildInfo{Main: main("(devel)")}, ""},
		{"dirty version string", debug.BuildInfo{Main: main("v0.97.0+dirty")}, ""},
	}
	for _, c := range cases {
		got, ok := buildInfoFingerprint(&c.bi)
		if c.want == "" {
			if ok {
				t.Errorf("%s: got %q, want fallback", c.name, got)
			}
			continue
		}
		if !ok || got != c.want {
			t.Errorf("%s: got %q (%v), want %q", c.name, got, ok, c.want)
		}
	}
}

// TestExecutableFingerprint: the test binary has no VCS stamp, so this is the
// path dirty and development builds take. The hash is memoized on disk and
// stable across calls.
func TestExecutableFingerprint(t *testing.T) {
	dir := t.TempDir()
	a, ok := executableFingerprint(dir)
	if !ok || !strings.HasPrefix(a, "exe:") || len(a) != len("exe:")+64 {
		t.Fatalf("fingerprint = %q, %v", a, ok)
	}
	memos, _ := filepath.Glob(filepath.Join(dir, "exe-*"))
	if len(memos) != 1 {
		t.Fatalf("want one memo file, got %v", memos)
	}
	// A memoized value is used as-is (proves the memo is read, not recomputed).
	if err := os.WriteFile(memos[0], []byte(strings.Repeat("0", 64)), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, _ := executableFingerprint(dir); b != "exe:"+strings.Repeat("0", 64) {
		t.Errorf("memo not used: %q", b)
	}
}

func TestToolchainFingerprint(t *testing.T) {
	got, ok := parseToolchainEnv([]byte(`{"GOARCH":"arm64","GOEXPERIMENT":"arenas","GOFLAGS":"","GOOS":"darwin","GOROOT":"/go","GOVERSION":"go1.27.3"}`))
	want := `GOARCH="arm64";GOEXPERIMENT="arenas";GOFLAGS="";GOOS="darwin";GOROOT="/go";GOVERSION="go1.27.3";`
	if !ok || got != want {
		t.Errorf("got %q (%v), want %q", got, ok, want)
	}
	other, _ := parseToolchainEnv([]byte(`{"GOARCH":"arm64","GOEXPERIMENT":"","GOFLAGS":"","GOOS":"darwin","GOROOT":"/go","GOVERSION":"go1.27.3"}`))
	if other == got {
		t.Error("GOEXPERIMENT must change the fingerprint")
	}
	if _, ok := parseToolchainEnv([]byte(`not json`)); ok {
		t.Error("bad output accepted")
	}
	// The real go command, as the tests run it.
	live, ok := toolchainFingerprint()
	if !ok || !strings.Contains(live, "GOVERSION=\"go") {
		t.Errorf("live toolchain fingerprint = %q, %v", live, ok)
	}
}

func TestEnvironment(t *testing.T) {
	env, ok := Environment(t.TempDir())
	if !ok || !strings.Contains(env, "build=exe:") || !strings.Contains(env, "toolchain=") {
		t.Errorf("Environment = %q, %v", env, ok)
	}
}
