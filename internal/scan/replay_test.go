package scan

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/collector"
	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
	"github.com/pilsu-choi/mcpgraph/policies"
)

// secretActions are agent actions against the fixture apps. Some carry
// fake secrets, so the leak test also covers Decision nodes and the audit
// log. Agent/App name servers and apps that exist in testdata/configs.
func secretActions() []policy.Action {
	return []policy.Action{
		// Hook path: Claude Code runs the demo command.
		{Source: model.SourceHook, Agent: "claude-code", App: "Claude Code", Tool: "Bash", Cwd: "/work/app",
			Text: `PGPASSWORD=FakePassw0rdXyz psql -h db -c "DROP TABLE users"`},
		{Source: model.SourceHook, Agent: "claude-code", App: "Claude Code", Tool: "Bash", Cwd: "/work/app",
			Text: `git push --force https://x:ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234@github.com/o/r`},
		{Source: model.SourceHook, Agent: "claude-code", App: "Claude Code", Tool: "Bash", Cwd: "/work/app", Text: "go test ./..."},
		// Proxy path: Claude Desktop's postgres server gets a query tool call.
		{Source: model.SourceProxy, Agent: "postgres", App: "Claude Desktop", Tool: "query",
			Args: map[string]any{"sql": "DELETE FROM users WHERE 1=1", "dsn": "postgresql://app:FakePassw0rdXyz@db/prod"}},
		{Source: model.SourceProxy, Agent: "github", App: "Claude Desktop", Tool: "create_issue",
			Args: map[string]any{"title": "hi", "token": "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234"}},
		// A server that is not in any config: decision without an edge.
		{Source: model.SourceProxy, Agent: "removed-server", Tool: "x", Args: map[string]any{"q": "1"}},
	}
}

// replayDecisions judges actions with the shipped policy, appends each
// decision to a fresh audit log (as the proxy and hook will) and returns
// its path.
func replayDecisions(t *testing.T, actions []policy.Action) string {
	t.Helper()
	p, err := policy.Parse(policies.Default)
	if err != nil {
		t.Fatal(err)
	}
	e, err := policy.New(p, policy.Options{Home: "/home/u"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), audit.FileName)
	l, err := audit.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, a := range actions {
		d := policy.Judge(context.Background(), e, a, time.Second)
		if err := l.Append(d); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// TestReplayDecisionsLinksGraph is the milestone 6 acceptance test: replay
// decisions through the audit log and check the graph shows them as
// Decision nodes with DECIDED_ON edges to the right server or app.
func TestReplayDecisionsLinksGraph(t *testing.T) {
	g, _ := scanFixtures(t)

	decisions := g.NodesOfKind(model.KindDecision)
	if len(decisions) != len(secretActions()) {
		t.Fatalf("graph has %d Decision nodes, want %d", len(decisions), len(secretActions()))
	}

	type want struct {
		verdict model.Verdict
		rule    string
		target  string // "App / server" for servers, app name for apps, "" for no edge
	}
	wants := map[string]want{ // keyed by tool
		"Bash/DROP":    {model.VerdictDeny, "block-destructive-sql", "Claude Code"},
		"Bash/push":    {model.VerdictAsk, "ask-git-force-push", "Claude Code"},
		"Bash/test":    {model.VerdictAllow, "", "Claude Code"},
		"query":        {model.VerdictDeny, "block-destructive-sql", "Claude Desktop / postgres"},
		"create_issue": {model.VerdictAllow, "", "Claude Desktop / github"},
		"x":            {model.VerdictAllow, "", ""},
	}
	for _, n := range decisions {
		d := n.Decision
		key := d.Tool
		if d.Tool == "Bash" {
			switch {
			case strings.Contains(d.Summary, "DROP"):
				key = "Bash/DROP"
			case strings.Contains(d.Summary, "push"):
				key = "Bash/push"
			default:
				key = "Bash/test"
			}
		}
		w, ok := wants[key]
		if !ok {
			t.Fatalf("unexpected decision %+v", d)
		}
		if d.Verdict != w.verdict || d.RuleID != w.rule {
			t.Errorf("%s: got %s/%q, want %s/%q", key, d.Verdict, d.RuleID, w.verdict, w.rule)
		}
		targets := g.Out(n.ID, model.EdgeDecidedOn)
		if w.target == "" {
			if len(targets) != 0 {
				t.Errorf("%s: want no DECIDED_ON edge, got %d", key, len(targets))
			}
			continue
		}
		if len(targets) == 0 {
			t.Errorf("%s: no DECIDED_ON edge, want one to %s", key, w.target)
			continue
		}
		for _, tg := range targets {
			got := ""
			switch tg.Kind {
			case model.KindMCPServer:
				got = tg.Server.App + " / " + tg.Server.Name
			case model.KindAIApp:
				got = tg.App.Name
			}
			if got != w.target {
				t.Errorf("%s: DECIDED_ON %s, want %s", key, got, w.target)
			}
		}
	}
}

// TestReplayIsDeterministic checks that scanning the same audit log twice
// gives the same Decision node IDs, so decisions keep their identity
// across scans like every other node.
func TestReplayIsDeterministic(t *testing.T) {
	home := testutil.FakeHome(t)
	path := replayDecisions(t, secretActions())
	ids := func() []string {
		g, err := Run(context.Background(), []collector.Collector{config.New(home, "linux"), audit.Collector{Path: path}})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, n := range g.NodesOfKind(model.KindDecision) {
			out = append(out, n.ID)
		}
		return out
	}
	a, b := ids(), ids()
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("ids %v vs %v", a, b)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("ids differ: %v vs %v", a, b)
		}
	}
}
