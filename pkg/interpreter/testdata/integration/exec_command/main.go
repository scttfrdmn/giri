// exec_command verifies that os/exec.Command and Cmd method intercepts work (#90).
//
// Expected: 0 violations.
package main

import "os/exec"

func main() {
	// exec.Command returns a non-nil *Cmd.
	cmd := exec.Command("echo", "hello")
	_ = cmd

	// String returns a human-readable representation.
	_ = cmd.String()

	// Output runs the command and captures stdout.
	out, err := cmd.Output()
	_ = out // []byte("output")
	_ = err // nil

	// CombinedOutput captures both stdout and stderr.
	cmd2 := exec.Command("ls", "-la")
	combined, err2 := cmd2.CombinedOutput()
	_ = combined
	_ = err2

	// Run executes without capturing output.
	cmd3 := exec.Command("true")
	err3 := cmd3.Run()
	_ = err3 // nil

	// Start + Wait execute asynchronously.
	cmd4 := exec.Command("sleep", "0")
	err4 := cmd4.Start()
	_ = err4 // nil
	err5 := cmd4.Wait()
	_ = err5 // nil

	// StdoutPipe / StderrPipe / StdinPipe return (ReadCloser/WriteCloser, nil).
	cmd5 := exec.Command("cat")
	stdout, err6 := cmd5.StdoutPipe()
	_ = stdout
	_ = err6
}
