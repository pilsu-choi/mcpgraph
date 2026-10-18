package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// RunCommand starts the MCP server argv as a child process and proxies
// between the client streams (normally the proxy's own stdin/stdout) and
// the child. The child's stderr goes straight to stderr: MCP servers may
// log there freely (research §A7).
//
// It returns the child's exit code. When ctx is cancelled (the client sent
// SIGTERM to the proxy), the child gets SIGTERM, then SIGKILL after a
// grace period. The proxy only ever signals its own child.
func (p *Proxy) RunCommand(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(argv) == 0 {
		return 1, errors.New("no server command given")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// CommandContext would send SIGKILL on cancel; ask politely first.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Stderr = stderr

	serverIn, err := cmd.StdinPipe()
	if err != nil {
		return 1, err
	}
	serverOut, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start MCP server %q: %w", argv[0], err)
	}

	// Run returns once the child's stdout is closed; Wait must only be
	// called after that, because Wait closes the pipe (os/exec docs).
	runErr := p.Run(ctx, stdin, stdout, serverIn, serverOut)
	waitErr := cmd.Wait()

	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
		return 0, runErr
	case errors.As(waitErr, &exitErr):
		return exitErr.ExitCode(), runErr
	default:
		return 1, errors.Join(runErr, waitErr)
	}
}
