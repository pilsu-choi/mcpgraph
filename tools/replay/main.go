// Command replay judges recorded agent actions with a policy and appends
// the decisions to an audit log, the way `mcpgraph proxy` and `mcpgraph
// hook` will. It feeds scripts/docs.sh and tests until those paths exist.
//
//	go run ./tools/replay -data-dir DIR < testdata/actions/demo.jsonl
//
// Input is JSON Lines, one policy.Action per line:
//
//	{"source":"hook","agent":"claude-code","app":"Claude Code","tool":"Bash","text":"ls","cwd":"/work/app"}
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
	"github.com/pilsu-choi/mcpgraph/policies"
)

// action is the JSON form of policy.Action. policy.Action has no JSON tags
// because it never leaves the process in production.
type action struct {
	Source string         `json:"source"`
	Agent  string         `json:"agent"`
	App    string         `json:"app"`
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	Text   string         `json:"text"`
	Cwd    string         `json:"cwd"`
	// Time, if set, is used as the decision time so output is stable.
	Time time.Time `json:"time"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "replay:", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir := flag.String("data-dir", "", "mcpgraph data directory (required)")
	home := flag.String("home", "", "home directory for ~ in the policy (default: $HOME)")
	flag.Parse()
	if *dataDir == "" {
		return fmt.Errorf("-data-dir is required")
	}
	if *home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		*home = h
	}
	p, err := policy.Parse(policies.Default)
	if err != nil {
		return err
	}
	log, err := audit.Open(filepath.Join(*dataDir, audit.FileName))
	if err != nil {
		return err
	}
	defer log.Close()

	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var a action
		if err := json.Unmarshal(sc.Bytes(), &a); err != nil {
			return fmt.Errorf("parse action: %w", err)
		}
		d, err := judge(p, *home, a)
		if err != nil {
			return err
		}
		if err := log.Append(d); err != nil {
			return err
		}
		printDecision(os.Stdout, d)
	}
	return sc.Err()
}

func judge(p *policy.Policy, home string, a action) (model.Decision, error) {
	opts := policy.Options{Home: home}
	if !a.Time.IsZero() {
		opts.Now = func() time.Time { return a.Time }
	}
	e, err := policy.New(p, opts)
	if err != nil {
		return model.Decision{}, err
	}
	return policy.Judge(context.Background(), e, policy.Action{
		Source: a.Source, Agent: a.Agent, App: a.App, Tool: a.Tool, Args: a.Args, Text: a.Text, Cwd: a.Cwd,
	}, time.Second), nil
}

func printDecision(w io.Writer, d model.Decision) {
	rule := d.RuleID
	if rule == "" {
		rule = "-"
	}
	fmt.Fprintf(w, "%-5s  %-22s  %-5s  %s\n", d.Verdict, rule, d.Source, d.Summary)
}
