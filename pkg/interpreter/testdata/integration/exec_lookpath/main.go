// exec_lookpath verifies that os/exec.LookPath and exec.CommandContext intercepts
// work (#90).
//
// Expected: 0 violations.
package main

import (
	"context"
	"os/exec"
)

func main() {
	// LookPath searches PATH for an executable.
	path, err := exec.LookPath("go")
	_ = path // "/usr/bin/go" (sentinel)
	_ = err  // nil

	// Non-existent executable: LookPath still returns success in our model.
	path2, err2 := exec.LookPath("nonexistent-tool")
	_ = path2
	_ = err2

	// CommandContext wraps Command with a cancellation context.
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go", "version")
	_ = cmd

	out, err3 := cmd.Output()
	_ = out
	_ = err3

	// Environ returns the command's environment.
	_ = cmd.Environ()
}
