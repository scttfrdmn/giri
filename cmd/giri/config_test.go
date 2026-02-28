package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scttfrdmn/giri/pkg/interpreter"
)

func TestLoadProjectConfig_Missing(t *testing.T) {
	// Change to a temp dir with no .giri.json.
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev) //nolint:errcheck
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	pc, err := loadProjectConfig()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if pc != nil {
		t.Fatalf("expected nil config, got %+v", pc)
	}
}

func TestLoadProjectConfig_Valid(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev) //nolint:errcheck
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	content := `{
		"format":   "sarif",
		"strategy": "pct",
		"seed":     42,
		"runs":     10,
		"race":     true,
		"unsafe":   false
	}`
	if err := os.WriteFile(filepath.Join(tmp, ".giri.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	pc, err := loadProjectConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pc == nil {
		t.Fatal("expected non-nil config")
	}
	if pc.Format == nil || *pc.Format != "sarif" {
		t.Errorf("format: got %v, want \"sarif\"", pc.Format)
	}
	if pc.Strategy == nil || *pc.Strategy != "pct" {
		t.Errorf("strategy: got %v, want \"pct\"", pc.Strategy)
	}
	if pc.Seed == nil || *pc.Seed != 42 {
		t.Errorf("seed: got %v, want 42", pc.Seed)
	}
	if pc.Runs == nil || *pc.Runs != 10 {
		t.Errorf("runs: got %v, want 10", pc.Runs)
	}
	if pc.Race == nil || !*pc.Race {
		t.Errorf("race: got %v, want true", pc.Race)
	}
	if pc.Unsafe == nil || *pc.Unsafe {
		t.Errorf("unsafe: got %v, want false", pc.Unsafe)
	}
}

func TestLoadProjectConfig_Invalid(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev) //nolint:errcheck
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, ".giri.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadProjectConfig()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestApplyProjectConfig(t *testing.T) {
	seed := int64(99)
	depth := 5
	strategy := "random"
	unsafe := false

	pc := &projectConfig{
		Seed:     &seed,
		Depth:    &depth,
		Strategy: &strategy,
		Unsafe:   &unsafe,
	}

	config := interpreter.DefaultConfig()
	applyProjectConfig(&config, pc)

	if config.RandomSeed != 99 {
		t.Errorf("seed: got %d, want 99", config.RandomSeed)
	}
	if config.BugDepth != 5 {
		t.Errorf("depth: got %d, want 5", config.BugDepth)
	}
	if config.ScheduleStrategy != interpreter.ScheduleRandom {
		t.Errorf("strategy: got %v, want ScheduleRandom", config.ScheduleStrategy)
	}
	if config.TrackUnsafe {
		t.Error("unsafe: expected false after applyProjectConfig")
	}
	// TrackArenas/TrackRaces should remain at default (true) since not set in pc.
	if !config.TrackArenas {
		t.Error("arena should still be enabled (not in pc)")
	}
}
