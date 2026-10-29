package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
)

// Environment returns the part of the cache key that identifies what produced
// a result, beyond the program's source and the analysis config (#263):
//
//   - the giri build: a clean VCS-stamped build's commit, else a released
//     module version, else a hash of the running executable. Any change to
//     giri's code therefore changes the key, with no version to bump by hand.
//   - the Go toolchain whose standard library is loaded and analyzed:
//     GOVERSION, GOROOT, GOOS, GOARCH, GOEXPERIMENT and GOFLAGS as reported by
//     `go env`. The module's go directive alone misses toolchain upgrades and
//     build-tag changes.
//
// ok is false when either part can't be determined; callers must then bypass
// the cache rather than risk serving a result from a different giri or
// toolchain. dir is the cache directory, used to memoize the executable hash.
// The result is computed once per process.
func Environment(dir string) (env string, ok bool) {
	envOnce.Do(func() {
		b, bok := buildFingerprint(dir)
		t, tok := toolchainFingerprint()
		envValue, envOK = "build="+b+"\ntoolchain="+t, bok && tok
	})
	return envValue, envOK
}

var (
	envOnce  sync.Once
	envValue string
	envOK    bool
)

// buildFingerprint identifies the running giri binary.
func buildFingerprint(dir string) (string, bool) {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if fp, ok := buildInfoFingerprint(bi); ok {
			return fp, true
		}
	}
	return executableFingerprint(dir)
}

// buildInfoFingerprint derives a fingerprint from embedded build information
// when it pins the source exactly: an unmodified VCS checkout, or a module
// version that names a release or a commit (go install pkg@version). Dirty
// checkouts, `go run`/`go test` builds and "(devel)" versions return false.
func buildInfoFingerprint(bi *debug.BuildInfo) (string, bool) {
	var rev string
	modified := false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if rev != "" && !modified {
		return "vcs:" + rev, true
	}
	if rev == "" { // no VCS stamp: the version comes from the module proxy
		if v := bi.Main.Version; v != "" && v != "(devel)" && !strings.Contains(v, "+dirty") {
			return "mod:" + bi.Main.Path + "@" + v, true
		}
	}
	return "", false
}

// executableFingerprint hashes the running executable. The hash is memoized in
// dir under the executable's path, size and modification time, so repeated
// runs of the same binary don't re-read it.
func executableFingerprint(dir string) (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	fi, err := os.Stat(exe)
	if err != nil {
		return "", false
	}
	id := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%d", exe, fi.Size(), fi.ModTime().UnixNano())))
	memo := filepath.Join(dir, "exe-"+hex.EncodeToString(id[:16]))
	if data, err := os.ReadFile(memo); err == nil && len(data) == 64 {
		return "exe:" + string(data), true
	}
	f, err := os.Open(exe)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }() // read-only
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false
	}
	sum := hex.EncodeToString(h.Sum(nil))
	_ = os.WriteFile(memo, []byte(sum), 0o644) // best-effort, like all cache I/O
	return "exe:" + sum, true
}

// toolchainVars are the go env settings that change which standard library
// source giri loads and how it is type-checked.
var toolchainVars = []string{"GOVERSION", "GOROOT", "GOOS", "GOARCH", "GOEXPERIMENT", "GOFLAGS"}

// toolchainFingerprint describes the go command packages.Load will use.
func toolchainFingerprint() (string, bool) {
	out, err := exec.Command("go", append([]string{"env", "-json"}, toolchainVars...)...).Output()
	if err != nil {
		return "", false
	}
	return parseToolchainEnv(out)
}

// parseToolchainEnv turns `go env -json` output into a canonical string.
func parseToolchainEnv(out []byte) (string, bool) {
	var vars map[string]string
	if err := json.Unmarshal(bytes.TrimSpace(out), &vars); err != nil || vars["GOVERSION"] == "" {
		return "", false
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%q;", k, vars[k])
	}
	return b.String(), true
}
