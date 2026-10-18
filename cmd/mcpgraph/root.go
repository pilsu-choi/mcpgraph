package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/pilsuchoi/mcpgraph/internal/collector"
	"github.com/pilsuchoi/mcpgraph/internal/collector/config"
	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/report"
	"github.com/pilsuchoi/mcpgraph/internal/scan"
)

// options holds flags shared by every subcommand.
type options struct {
	home    string
	verbose bool
}

// newRootCmd builds the command tree. Building it in a function (instead of
// package-level variables) keeps global state out and lets tests create a
// fresh tree each time.
func newRootCmd() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:           "mcpgraph",
		Short:         "Map AI apps, their MCP servers and what those servers can reach",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelWarn
			if opts.verbose {
				level = slog.LevelDebug
			}
			// The root logger is the only global state we allow.
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
		},
	}
	root.PersistentFlags().StringVar(&opts.home, "home", "", "home directory to scan (default: current user's home)")
	root.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "debug logging")

	root.AddCommand(newScanCmd(opts))
	return root
}

func newScanCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan this machine and print an inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := runScan(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return report.Inventory(cmd.OutOrStdout(), g)
		},
	}
}

// runScan runs every collector once and returns the graph.
func runScan(ctx context.Context, opts *options) (*model.Graph, error) {
	home := opts.home
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("find home directory: %w", err)
		}
		home = h
	}
	collectors := []collector.Collector{
		collector.Host{},
		config.New(home, runtime.GOOS),
	}
	return scan.Run(ctx, collectors)
}
