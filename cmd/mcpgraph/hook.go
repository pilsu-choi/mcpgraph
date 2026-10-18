package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/hook"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
)

func newHookCmd(opts *options) *cobra.Command {
	var (
		policyPath  string
		timeout     time.Duration
		printConfig bool
		bin         string
	)
	cmd := &cobra.Command{
		Use:   "hook <claude-code|codex>",
		Short: "Judge one coding-agent tool use read from stdin (PreToolUse hook)",
		Long: `Run as a PreToolUse hook of a coding agent. Reads one JSON payload on stdin,
judges it with the same policy as the proxy, appends the decision to the
audit log and answers in the agent's format.

An allowed call prints nothing and exits 0, so the agent's own permission
prompt still applies. A denied call prints a deny answer. A call that needs
approval prints an ask answer (Claude Code) or a deny that explains how to
approve (Codex, which has no ask). If anything fails (bad payload, bad
policy, unknown agent) the hook writes the reason to stderr and exits 2,
which blocks the call: fail closed.

--print-config prints the hook configuration to add to the agent's config
file. It writes nothing; only 'mcpgraph install' edits app configs.`,
		Example: `  mcpgraph hook claude-code --print-config`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			adapter, ok := hook.Lookup(args[0])
			if !ok {
				return blockHook(cmd, fmt.Errorf("unknown agent %q (want claude-code or codex)", args[0]))
			}
			if printConfig {
				return printHookConfig(cmd, adapter, bin)
			}
			home, err := opts.homeDir()
			if err != nil {
				return blockHook(cmd, err)
			}
			// No tool inventory in the hook: unknown_tool is proxy-only.
			engine, err := buildEngine(policyPath, home, nil)
			if err != nil {
				return blockHook(cmd, err)
			}
			return runHook(cmd, opts, adapter, engine, timeout)
		},
	}
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy file (default: the built-in policies/default.yaml)")
	cmd.Flags().DurationVar(&timeout, "decision-timeout", 2*time.Second, "block the call if the policy takes longer than this")
	cmd.Flags().BoolVar(&printConfig, "print-config", false, "print the hook config for the agent and exit")
	cmd.Flags().StringVar(&bin, "bin", "", "mcpgraph path for --print-config (default: this binary)")
	return cmd
}

// runHook handles one payload: parse, judge, audit, answer.
func runHook(cmd *cobra.Command, opts *options, adapter hook.Adapter, engine policy.Engine, timeout time.Duration) error {
	action, err := adapter.Parse(cmd.InOrStdin())
	if err != nil {
		return blockHook(cmd, err)
	}
	d := policy.Judge(cmd.Context(), engine, action, timeout)
	appendAudit(opts, d)
	// An allow encodes to nothing: print nothing, exit 0.
	if _, err := cmd.OutOrStdout().Write(adapter.Encode(d)); err != nil {
		return blockHook(cmd, fmt.Errorf("write answer: %w", err))
	}
	return nil
}

// appendAudit records d. A missing audit log is a warning, not a reason to
// stop answering the agent.
func appendAudit(opts *options, d model.Decision) {
	path, err := opts.auditPath()
	if err != nil {
		slog.Warn("mcpgraph hook: no audit log", "err", err)
		return
	}
	log, err := audit.Open(path)
	if err != nil {
		slog.Warn("mcpgraph hook: no audit log", "err", err)
		return
	}
	defer log.Close()
	if err := log.Append(d); err != nil {
		slog.Warn("mcpgraph hook: audit append failed", "err", err)
	}
}

// blockHook blocks the tool call: exit code 2 with the reason on stderr
// works for both agents. err must not contain tool input.
func blockHook(cmd *cobra.Command, err error) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "mcpgraph hook: blocked (rule %s): %v\n", policy.FailClosedRuleID, err)
	return exitError{code: 2}
}

func printHookConfig(cmd *cobra.Command, adapter hook.Adapter, bin string) error {
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find mcpgraph binary (use --bin): %w", err)
		}
		bin = exe
	}
	b, err := adapter.ConfigSnippet(bin)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", b)
	return err
}
