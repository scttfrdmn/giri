package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scttfrdmn/giri/pkg/cache"
	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
)

func TestKeyStability(t *testing.T) {
	k1 := cache.Key("srchash", "fp", "0.95.0", "go1.23")
	k2 := cache.Key("srchash", "fp", "0.95.0", "go1.23")
	if k1 != k2 {
		t.Errorf("identical inputs must yield identical keys: %s vs %s", k1, k2)
	}
	// Each component change must change the key.
	cases := []struct {
		name             string
		src, fp, gv, gov string
	}{
		{"source", "OTHER", "fp", "0.95.0", "go1.23"},
		{"fingerprint", "srchash", "OTHER", "0.95.0", "go1.23"},
		{"giriVersion", "srchash", "fp", "0.96.0", "go1.23"},
		{"goVersion", "srchash", "fp", "0.95.0", "go1.24"},
	}
	for _, c := range cases {
		if got := cache.Key(c.src, c.fp, c.gv, c.gov); got == k1 {
			t.Errorf("%s change must alter the key, but it did not", c.name)
		}
	}
}

func TestFingerprintDistinguishesConfig(t *testing.T) {
	base := interpreter.DefaultConfig()
	fpBase := cache.Fingerprint(base)

	withTrunc := base
	withTrunc.TrackTruncation = !base.TrackTruncation
	if cache.Fingerprint(withTrunc) == fpBase {
		t.Error("toggling TrackTruncation must change the fingerprint")
	}

	// Verbose is not analysis-affecting → must NOT change the fingerprint.
	withVerbose := base
	withVerbose.Verbose = !base.Verbose
	if cache.Fingerprint(withVerbose) != fpBase {
		t.Error("toggling Verbose must not change the fingerprint")
	}
}

func TestStoreLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	key := "abc123"
	entry := &cache.Entry{
		Active: []report.Finding{
			{Category: "integer-truncation", Severity: report.SeverityWarning, Message: "m", Location: "a.go:5"},
		},
		Suppressed:      []report.Finding{{Category: "out-of-bounds", Suppressed: true}},
		SuppressedCount: 1,
	}
	if err := cache.Store(dir, key, entry); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, ok := cache.Load(dir, key)
	if !ok {
		t.Fatal("expected cache hit after store")
	}
	if len(got.Active) != 1 || got.Active[0].Category != "integer-truncation" {
		t.Errorf("active findings not round-tripped: %+v", got.Active)
	}
	if len(got.Suppressed) != 1 || !got.Suppressed[0].Suppressed {
		t.Errorf("suppressed findings not round-tripped: %+v", got.Suppressed)
	}
	if got.SuppressedCount != 1 {
		t.Errorf("SuppressedCount: want 1, got %d", got.SuppressedCount)
	}
}

func TestLoadMiss(t *testing.T) {
	dir := t.TempDir()
	if _, ok := cache.Load(dir, "nonexistent"); ok {
		t.Error("expected miss for absent key")
	}
}

func TestLoadCorruptEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	key := "corrupt"
	if err := os.WriteFile(filepath.Join(dir, key+".json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Load(dir, key); ok {
		t.Error("corrupt entry must be treated as a miss, not a hit")
	}
}

func TestDirHonorsEnv(t *testing.T) {
	base := t.TempDir()
	t.Setenv("GIRI_CACHE", base)
	dir, ok := cache.Dir()
	if !ok {
		t.Fatal("Dir should succeed with GIRI_CACHE set")
	}
	if filepath.Dir(dir) != base {
		t.Errorf("Dir %q should be under GIRI_CACHE %q", dir, base)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Dir should be created: %v", err)
	}
}
