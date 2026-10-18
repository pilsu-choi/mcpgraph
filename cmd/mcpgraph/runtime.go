package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// errNotImplemented marks subcommands whose milestone is not done yet.
// They exist now so the CLI shape matches AGENTS.md from the start.
var errNotImplemented = errors.New("not implemented yet")

func notYet(milestone string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("%s: %w (milestone %s)", cmd.Name(), errNotImplemented, milestone)
	}
}

func newHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook <agent>",
		Short: "Judge one coding-agent tool use read from stdin (agents: claude-code)",
		Args:  cobra.ExactArgs(1),
		RunE:  notYet("m9"),
	}
}
