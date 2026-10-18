package main

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
	"github.com/pilsu-choi/mcpgraph/internal/proxy"
	"github.com/pilsu-choi/mcpgraph/policies"
)

func newProxyCmd(opts *options) *cobra.Command {
	var (
		server     string
		app        string
		policyPath string
		timeout    time.Duration
	)
	cmd := &cobra.Command{
		Use:   "proxy [flags] -- <server command> [args...]",
		Short: "Run an stdio MCP server behind the policy engine",
		Long: `Run an stdio MCP server as a child process and sit between it and the AI app.

Every message is passed through unchanged except tools/call requests, which
are judged by the policy first. Allowed calls are forwarded; denied calls
(and calls that need approval) are answered with an error result that the
model can read, and never reach the server. Every decision is appended to
the audit log. If the policy cannot be loaded or a decision fails, the
tool call is denied (fail closed).

stdout carries the MCP protocol, so mcpgraph logs only to stderr.`,
		Example: `  mcpgraph proxy --server postgres --app "Claude Desktop" -- npx -y @modelcontextprotocol/server-postgres postgresql://localhost/dev`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if server == "" {
				server = filepath.Base(args[0])
			}
			home, err := opts.homeDir()
			if err != nil {
				return err
			}

			p := proxy.New(proxy.Config{
				Server: server, App: app, Cwd: proxy.Cwd(), Timeout: timeout, Logger: slog.Default(),
			})
			p.SetEngine(loadEngine(policyPath, home, p.KnownTool))

			if path, err := opts.auditPath(); err != nil {
				slog.Warn("mcpgraph proxy: no audit log", "err", err)
			} else if log, err := audit.Open(path); err != nil {
				slog.Warn("mcpgraph proxy: no audit log", "err", err)
			} else {
				defer log.Close()
				p.SetAudit(log)
			}

			code, err := p.RunCommand(cmd.Context(), args, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				slog.Warn("mcpgraph proxy", "err", err)
			}
			if code != 0 {
				return exitError{code: code}
			}
			return nil
		},
	}
	// Stop flag parsing at the server command, so its own flags
	// (`npx -y ...`) are not taken as mcpgraph flags even without "--".
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&server, "server", "", "MCP server name as in the AI app config (default: command base name)")
	cmd.Flags().StringVar(&app, "app", "", "AI app that runs this server, e.g. Cursor")
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy file (default: the built-in policies/default.yaml)")
	cmd.Flags().DurationVar(&timeout, "decision-timeout", 2*time.Second, "deny a tool call if the policy takes longer than this")
	return cmd
}

// loadEngine builds the policy engine. On any error it returns an engine
// that fails every decision, so the proxy still starts (other MCP traffic
// keeps working) but denies every tool call: hard rule 4, fail closed.
func loadEngine(path, home string, known func(server, tool string) bool) policy.Engine {
	var (
		p   *policy.Policy
		err error
	)
	if path == "" {
		p, err = policy.Parse(policies.Default)
	} else {
		p, err = policy.LoadFile(path)
	}
	if err == nil {
		var e *policy.RuleEngine
		if e, err = policy.New(p, policy.Options{Home: home, KnownTool: known}); err == nil {
			return e
		}
	}
	err = fmt.Errorf("load policy: %w", err)
	slog.Error("mcpgraph proxy: every tool call will be denied", "err", err)
	return proxy.FailingEngine(err)
}
