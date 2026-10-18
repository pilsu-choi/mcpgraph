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
	"github.com/pilsuchoi/mcpgraph/internal/collector/process"
	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/report"
	"github.com/pilsuchoi/mcpgraph/internal/scan"
)

// options holds flags shared by every subcommand.
type options struct {
	home        string
	verbose     bool
	noProcesses bool
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
	root.PersistentFlags().BoolVar(&opts.noProcesses, "no-processes", false, "skip process discovery")

	root.AddCommand(newScanCmd(opts), newExportCmd(opts))
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

func newExportCmd(opts *options) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Scan and print the graph as JSON or Mermaid",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := runScan(cmd.Context(), opts)
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return report.JSON(cmd.OutOrStdout(), g)
			case "mermaid":
				return report.Mermaid(cmd.OutOrStdout(), g)
			default:
				return fmt.Errorf("unknown format %q (want json or mermaid)", format)
			}
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "json", "output format: json or mermaid")
	return cmd
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
	if !opts.noProcesses {
		collectors = append(collectors, process.New())
	}
	return scan.Run(ctx, collectors)
}
