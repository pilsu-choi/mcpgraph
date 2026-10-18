// Command mcpgraph scans this machine for AI apps and MCP servers and
// reports what they can reach.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// NotifyContext returns a context that is cancelled on Ctrl-C, so a
	// running scan stops cleanly. stop releases the signal handler.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := newRootCmd().ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(exitCode(err))
	}
}

// exitCode maps an error returned by a command to a process exit code and
// prints it, unless the error only carries an exit code.
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

// exitError carries a process exit code through cobra's error return
// without printing anything extra.
type exitError struct {
	code int
}

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }
