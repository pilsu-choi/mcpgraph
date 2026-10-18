package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/report"
)

func newEventsCmd(opts *options) *cobra.Command {
	var (
		f       audit.Filter
		since   string
		verdict string
		format  string
	)
	cmd := &cobra.Command{
		Use:   "events",
		Short: "List and filter runtime decisions from the audit log",
		Example: `  mcpgraph events                       # newest 50 decisions
  mcpgraph events --verdict deny --since 24h
  mcpgraph events --rule block-destructive-sql -f jsonl | jq .`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if f.Since, err = parseSince(since, time.Now()); err != nil {
				return err
			}
			switch v := model.Verdict(strings.ToLower(verdict)); v {
			case "", model.VerdictAllow, model.VerdictDeny, model.VerdictAsk:
				f.Verdict = v
			default:
				return fmt.Errorf("unknown verdict %q (want allow, deny or ask)", verdict)
			}
			if f.Source != "" && f.Source != model.SourceProxy && f.Source != model.SourceHook {
				return fmt.Errorf("unknown source %q (want proxy or hook)", f.Source)
			}
			path, err := opts.auditPath()
			if err != nil {
				return err
			}
			res, err := audit.ReadFile(path, f)
			if err != nil {
				return err
			}
			if res.Skipped > 0 {
				slog.Warn("skipped malformed audit lines", "path", path, "count", res.Skipped)
			}
			switch format {
			case "table":
				return report.Events(cmd.OutOrStdout(), res.Decisions)
			case "jsonl":
				return report.EventsJSONL(cmd.OutOrStdout(), res.Decisions)
			default:
				return fmt.Errorf("unknown format %q (want table or jsonl)", format)
			}
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&since, "since", "", "only decisions after this: a duration (30m, 24h, 7d) or an RFC 3339 time")
	fl.StringVar(&verdict, "verdict", "", "allow, deny or ask")
	fl.StringVar(&f.Source, "source", "", "proxy or hook")
	fl.StringVar(&f.Agent, "agent", "", "agent or MCP server name")
	fl.StringVar(&f.Tool, "tool", "", "tool name, e.g. Bash or query")
	fl.StringVar(&f.RuleID, "rule", "", "policy rule ID")
	fl.IntVarP(&f.Limit, "limit", "n", 50, "newest N decisions (0 = all)")
	fl.StringVarP(&format, "format", "f", "table", "output format: table or jsonl")
	return cmd
}

// parseSince accepts "", a Go duration ("90m", "24h"), a number of days
// ("7d") or an RFC 3339 timestamp.
func parseSince(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err == nil && n >= 0 && fmt.Sprint(n) == days {
			return now.AddDate(0, 0, -n), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("--since %q: want a duration (24h, 7d) or an RFC 3339 time", s)
}
