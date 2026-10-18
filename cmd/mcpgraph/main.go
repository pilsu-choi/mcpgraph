// Command mcpgraph scans this machine for AI apps and MCP servers and
// reports what they can reach.
package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	// Execute returns an exitError when a command wants a specific exit
	// code (for example 2 when a high-severity finding exists).
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}

// exitCode maps an error returned by a command to a process exit code.
func exitCode(err error) int {
	// errors.As walks the chain of wrapped errors (like isinstance on
	// each cause) and fills e if one of them is an exitError.
	var e exitError
	if errors.As(err, &e) {
		return e.code
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	return 1
}

// exitError carries a process exit code through cobra's error return.
type exitError struct {
	code int
	msg  string
}

func (e exitError) Error() string { return fmt.Sprintf("exit %d: %s", e.code, e.msg) }
