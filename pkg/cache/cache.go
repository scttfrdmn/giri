// Package cache provides an on-disk result cache for Giri analysis (#231).
//
// Giri interprets whole programs starting from a main package, so the cache
// unit is one entry per program (per main package), keyed by the hash of that
// program's transitive source closure plus the analysis configuration and the
// Giri/Go versions. On a cache hit the interpreter is skipped entirely and the
// stored findings are replayed into the report.
//
// The cache is only sound for deterministic single-run analysis: PCT/random
// scheduling produces a seed-dependent union of violations that varies between
// runs, so callers must bypass the cache for those strategies.
//
// All cache I/O is best-effort. A missing, unreadable, or unwritable cache
// directory disables caching silently — analysis correctness never depends on
// the cache.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

// dirVersion namespaces the on-disk cache layout. Bump when the Entry schema
// changes incompatibly so old entries are ignored rather than mis-decoded.
const dirVersion = "v1"

// Entry is the cached result of interpreting one program. It stores the
// rendered findings (not the raw []error, which is not round-trippable) — this
// is exactly what the reporter consumes.
type Entry struct {
	Active     []report.Finding `json:"active"`
	Suppressed []report.Finding `json:"suppressed"`
	// SuppressedCount mirrors RunResult.SuppressedCount for -v reporting.
	SuppressedCount int `json:"suppressed_count"`
	// MemStats is the memory profile from the interpreted run, cached so a hit
	// reproduces the same report byte-for-byte (same source → same profile).
	MemStats shadow.MemoryStats `json:"mem_stats"`
}

// Dir returns the directory used to store cache entries and whether caching is
// available. It honors $GIRI_CACHE; otherwise it uses the user cache dir
// (os.UserCacheDir) under giri/<dirVersion>. The directory is created on demand.
// If no writable location can be resolved, ok is false and callers should treat
// the cache as disabled.
func Dir() (dir string, ok bool) {
	base := os.Getenv("GIRI_CACHE")
	if base == "" {
		ucd, err := os.UserCacheDir()
		if err != nil {
			return "", false
		}
		base = filepath.Join(ucd, "giri")
	}
	dir = filepath.Join(base, dirVersion)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false
	}
	return dir, true
}

// Fingerprint returns a stable string capturing the analysis-affecting fields
// of cfg. Two configs that would produce different findings yield different
// fingerprints; fields that do not affect results (Verbose, and the
// non-serializable Intercepts callbacks) are excluded. RandomSeed/BugDepth are
// included for completeness but only matter under non-deterministic scheduling,
// which bypasses the cache anyway.
func Fingerprint(cfg interpreter.Config) string {
	return fmt.Sprintf(
		"unsafe=%t;arenas=%t;races=%t;init=%t;trunc=%t;dclose=%t;maxsteps=%d;maxgor=%d;sched=%d;seed=%d;depth=%d",
		cfg.TrackUnsafe, cfg.TrackArenas, cfg.TrackRaces, cfg.TrackInit,
		cfg.TrackTruncation, cfg.TrackDoubleClose,
		cfg.MaxSteps, cfg.MaxGoroutines, cfg.ScheduleStrategy, cfg.RandomSeed, cfg.BugDepth,
	)
}

// Key derives the cache key for a program from its transitive source hash, the
// analysis config fingerprint, and the Giri and Go versions. Any change to a
// reachable source file, an analysis-affecting config field, or the tool/Go
// version yields a different key.
func Key(sourceHash, configFingerprint, giriVersion, goVersion string) string {
	h := sha256.New()
	// Length-prefix each component so distinct field boundaries can't collide.
	// (hash.Hash.Write never returns an error.)
	for _, part := range []string{sourceHash, configFingerprint, giriVersion, goVersion} {
		_, _ = io.WriteString(h, strconv.Itoa(len(part))+":"+part+"\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Load reads the cache entry for key from dir. ok is false when the entry is
// absent or unreadable/corrupt (treated as a miss, never an error).
func Load(dir, key string) (entry *Entry, ok bool) {
	data, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		return nil, false
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, false // corrupt entry → miss
	}
	return &e, true
}

// Store writes entry under key in dir. It is best-effort: a write error is
// returned for optional -v logging but is otherwise non-fatal.
func Store(dir, key string, entry *Entry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	// Write to a temp file then rename for atomicity, so a concurrent reader
	// never sees a partially written entry.
	path := filepath.Join(dir, key+".json")
	tmp, err := os.CreateTemp(dir, key+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
