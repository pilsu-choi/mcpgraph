package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pilsu-choi/mcpgraph/internal/install"
)

func newInstallCmd(opts *options) *cobra.Command {
	var (
		apps, servers []string
		policyPath    string
		bin           string
		yes           bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Rewrite AI app configs so stdio MCP servers run through the proxy",
		Long: `Rewrite AI app MCP configs so each stdio server runs as
  mcpgraph proxy --server NAME --app APP -- <original command>

Without --yes this is a dry run: it prints what would change and writes
nothing. With --yes every file is backed up into the data directory first.
Only "command" and "args" of each wrapped server change; every other byte
of the file stays as it was. Undo with: mcpgraph uninstall --yes`,
		Example: `  mcpgraph install                      # show the diff
  mcpgraph install --yes                # apply it
  mcpgraph install --app Cursor --server github --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := opts.homeDir()
			if err != nil {
				return err
			}
			dataDir, err := opts.dataPath()
			if err != nil {
				return err
			}
			if bin == "" {
				if bin, err = selfPath(); err != nil {
					return err
				}
			}
			var proxyFlags []string
			if opts.dataDir != "" {
				abs, err := filepath.Abs(opts.dataDir)
				if err != nil {
					return err
				}
				proxyFlags = append(proxyFlags, "--data-dir", abs)
			}
			if policyPath != "" {
				abs, err := filepath.Abs(policyPath)
				if err != nil {
					return err
				}
				proxyFlags = append(proxyFlags, "--policy", abs)
			}

			changes, skips, err := install.Plan(install.Options{
				Home: home, GOOS: runtime.GOOS, Bin: bin, Apps: apps, Servers: servers, ProxyFlags: proxyFlags,
			})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			printPlan(out, changes, skips)
			if len(changes) == 0 {
				fmt.Fprintln(out, "Nothing to install.")
				return nil
			}
			for _, fc := range changes {
				fmt.Fprintln(out)
				if err := fc.Diff(out); err != nil {
					return err
				}
			}
			fmt.Fprintln(out)
			if !yes {
				fmt.Fprintln(out, "Dry run: nothing was written. Run again with --yes to apply.")
				return nil
			}
			recs, err := install.Apply(dataDir, changes, time.Now())
			for _, r := range recs {
				fmt.Fprintf(out, "wrote %s (backup: %s)\n", r.Real, r.Backup)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(out, "Done. Restart the AI apps so they start the servers through the proxy.")
			fmt.Fprintln(out, "Undo with: mcpgraph uninstall --yes")
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&apps, "app", nil, `only these apps, e.g. --app Cursor --app "Claude Desktop"`)
	cmd.Flags().StringSliceVar(&servers, "server", nil, "only servers with these names")
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy file the proxy should use (default: built-in)")
	cmd.Flags().StringVar(&bin, "bin", "", "path of the mcpgraph binary to write into configs (default: this binary)")
	cmd.Flags().BoolVar(&yes, "yes", false, "write the changes (default: dry run)")
	return cmd
}

func newUninstallCmd(opts *options) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Undo mcpgraph install, restoring the backups",
		Long: `Undo every mcpgraph install, newest first.

A file that is exactly as install left it is restored from its backup, byte
for byte. A file that changed since (apps rewrite their own configs) is not
overwritten with the old backup; instead the proxy wrapping is removed in
place and the other changes are kept. Without --yes this is a dry run.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := opts.dataPath()
			if err != nil {
				return err
			}
			steps, err := install.PlanUninstall(dataDir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(steps) == 0 {
				fmt.Fprintln(out, "Nothing to uninstall: no install is recorded in", dataDir)
				return nil
			}
			for _, s := range steps {
				switch s.Mode {
				case install.ModeRestore:
					fmt.Fprintf(out, "%s: restore the backup from %s (byte for byte)\n", s.Record.Real, s.Record.Time.Format(time.RFC3339))
				case install.ModeUnwrap:
					fmt.Fprintf(out, "%s: changed since install; remove the proxy wrapping in place and keep the other changes\n", s.Record.Real)
				case install.ModeGone:
					fmt.Fprintf(out, "%s: file no longer exists; forget it\n", s.Record.Real)
				}
			}
			for _, s := range steps {
				if len(s.Edits) == 0 {
					continue
				}
				fmt.Fprintln(out)
				if err := s.Diff(out); err != nil {
					return err
				}
			}
			fmt.Fprintln(out)
			if !yes {
				fmt.Fprintln(out, "Dry run: nothing was written. Run again with --yes to apply.")
				return nil
			}
			if err := install.ApplyUninstall(dataDir, steps); err != nil {
				return err
			}
			fmt.Fprintln(out, "Done. Restart the AI apps so they start the servers directly again.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "write the changes (default: dry run)")
	return cmd
}

func printPlan(out io.Writer, changes []install.FileChange, skips []install.Skip) {
	n := 0
	for _, fc := range changes {
		n += len(fc.Servers)
	}
	fmt.Fprintf(out, "Wrap %d stdio MCP server(s) in %d file(s) with mcpgraph proxy:\n", n, len(changes))
	for _, fc := range changes {
		fmt.Fprintf(out, "  %s  %s\n", fc.App, fc.Real)
		for _, s := range fc.Servers {
			fmt.Fprintf(out, "    + %s\n", s)
		}
	}
	if len(skips) > 0 {
		fmt.Fprintln(out, "Left alone:")
		for _, s := range skips {
			where := s.Path
			if s.Server != "" {
				where += "  " + s.Server
			}
			fmt.Fprintf(out, "  %s  %s: %s\n", s.App, where, s.Reason)
		}
	}
}

// selfPath returns the absolute path of the running mcpgraph binary. It
// refuses a `go run` build, whose binary is deleted when go run exits.
func selfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find the mcpgraph binary: %w (pass --bin)", err)
	}
	if strings.Contains(exe, "go-build") {
		return "", errors.New("this is a temporary `go run` binary; build or install mcpgraph and run that, or pass --bin")
	}
	if base := strings.TrimSuffix(filepath.Base(exe), ".exe"); base != "mcpgraph" {
		return "", fmt.Errorf("binary is named %q; scans recognise proxied servers only when it is named mcpgraph (pass --bin)", base)
	}
	return exe, nil
}
