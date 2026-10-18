package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRootCmd builds the command tree. Building it in a function (instead of
// package-level variables) keeps global state out and lets tests create a
// fresh tree each time.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "mcpgraph",
		Short:         "Map AI apps, their MCP servers and what those servers can reach",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newScanCmd())
	return root
}

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan this machine and print an inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "scan: not implemented yet")
			return nil
		},
	}
}
