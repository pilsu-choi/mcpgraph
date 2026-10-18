package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/collector"
	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
	"github.com/pilsu-choi/mcpgraph/internal/collector/process"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/posture"
	"github.com/pilsu-choi/mcpgraph/internal/report"
	"github.com/pilsu-choi/mcpgraph/internal/scan"
)

// options holds flags shared by every subcommand.
type options struct {
	home        string
	verbose     bool
	noProcesses bool
	dataDir     string
	decisions   int
}

// homeDir returns --home, or the current user's home directory.
func (o *options) homeDir() (string, error) {
	if o.home != "" {
		return o.home, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return h, nil
}

// dataPath returns mcpgraph's data directory: --data-dir, else the
// default. With --home set, $XDG_DATA_HOME is ignored so a fake home never
// touches the real user's data.
func (o *options) dataPath() (string, error) {
	if o.dataDir != "" {
		return o.dataDir, nil
	}
	home, err := o.homeDir()
	if err != nil {
		return "", err
	}
	if o.home != "" {
		return filepath.Join(home, ".local", "share", "mcpgraph"), nil
	}
	return audit.DefaultDataDir(home), nil
}

// auditPath returns the audit log path inside the data directory.
func (o *options) auditPath() (string, error) {
	dir, err := o.dataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, audit.FileName), nil
}

// newRootCmd builds the command tree. Building it in a function (instead of
// package-level variables) keeps global state out and lets tests create a
// fresh tree each time.
func newRootCmd() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:           "mcpgraph",
		Short:         "Watch and control what AI agents do on this machine",
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
	root.PersistentFlags().StringVar(&opts.dataDir, "data-dir", "", "mcpgraph data directory holding audit.jsonl (default: ~/.local/share/mcpgraph, macOS ~/Library/Application Support/mcpgraph)")
	root.PersistentFlags().IntVar(&opts.decisions, "decisions", audit.DefaultGraphLimit, "recent audit decisions to add to the graph (0 = none)")

	root.AddCommand(newScanCmd(opts), newReportCmd(opts), newExportCmd(opts))
	root.AddCommand(newProxyCmd(opts), newHookCmd(), newInstallCmd(opts), newUninstallCmd(opts), newEventsCmd(opts))
	return root
}

func newScanCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan this machine and print an inventory and findings",
		Long:  "Scan this machine and print an inventory and findings.\nExit code 2 means at least one high-severity finding.",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := runScan(cmd.Context(), opts)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if err := report.Inventory(out, res.graph); err != nil {
				return err
			}
			fmt.Fprintln(out)
			if err := report.Findings(out, res.findings); err != nil {
				return err
			}
			return exitForFindings(res.findings)
		},
	}
}

func newReportCmd(opts *options) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Scan and print only the findings (exit code 2 if any is high)",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := runScan(cmd.Context(), opts)
			if err != nil {
				return err
			}
			switch format {
			case "table":
				err = report.Findings(cmd.OutOrStdout(), res.findings)
			case "json":
				// Always emit a JSON array, never null.
				findings := res.findings
				if findings == nil {
					findings = []model.Finding{}
				}
				err = report.JSON(cmd.OutOrStdout(), findings)
			default:
				err = fmt.Errorf("unknown format %q (want table or json)", format)
			}
			if err != nil {
				return err
			}
			return exitForFindings(res.findings)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table or json")
	return cmd
}

// exitForFindings returns exit code 2 when there is a high finding, so the
// command can gate a CI job.
func exitForFindings(findings []model.Finding) error {
	if posture.HasHigh(findings) {
		return exitError{code: 2}
	}
	return nil
}

func newExportCmd(opts *options) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Scan and print the graph as JSON or Mermaid",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := runScan(cmd.Context(), opts)
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return report.JSON(cmd.OutOrStdout(), res.graph)
			case "mermaid":
				return report.Mermaid(cmd.OutOrStdout(), res.graph)
			default:
				return fmt.Errorf("unknown format %q (want json or mermaid)", format)
			}
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "json", "output format: json or mermaid")
	return cmd
}

// scanResult is the output of one scan.
type scanResult struct {
	graph    *model.Graph
	findings []model.Finding
}

// runScan runs every collector once, then the rules.
func runScan(ctx context.Context, opts *options) (scanResult, error) {
	home, err := opts.homeDir()
	if err != nil {
		return scanResult{}, err
	}
	collectors := []collector.Collector{
		collector.Host{},
		config.New(home, runtime.GOOS),
	}
	if !opts.noProcesses {
		collectors = append(collectors, process.New())
	}
	if opts.decisions > 0 {
		path, err := opts.auditPath()
		if err != nil {
			return scanResult{}, err
		}
		collectors = append(collectors, audit.Collector{Path: path, Limit: opts.decisions})
	}
	g, err := scan.Run(ctx, collectors)
	if err != nil {
		return scanResult{}, err
	}
	for _, w := range g.Warnings {
		slog.Warn(w)
	}
	return scanResult{graph: g, findings: posture.Evaluate(g, posture.Default(home))}, nil
}
