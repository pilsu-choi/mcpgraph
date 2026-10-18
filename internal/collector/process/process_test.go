package process

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// fakeTable is a small process tree:
//
//	1 init
//	├── 100 claude                     (AI app)
//	│   └── 101 npm exec @playwright/mcp@latest
//	│       └── 102 sh -c playwright-mcp
//	│           └── 103 node .../playwright-mcp
//	├── 200 bash
//	│   └── 201 grep mcp               (ignored tool)
//	├── 300 node /opt/github-mcp-server --token ghp_...   (MCP-like, orphan)
//	└── 400 sshd                       (uninteresting)
var fakeTable = []Info{
	{PID: 1, PPID: 0, Name: "init", Args: []string{"/sbin/init"}},
	{PID: 100, PPID: 1, Name: "claude", Args: []string{"claude"}, CreateTime: 1000},
	{PID: 101, PPID: 100, Name: "npm exec @playw", Args: []string{"npm", "exec", "@playwright/mcp@latest"}},
	{PID: 102, PPID: 101, Name: "sh", Args: []string{"sh", "-c", "playwright-mcp"}},
	{PID: 103, PPID: 102, Name: "node", Args: []string{"node", "/home/u/.npm/_npx/abc/node_modules/.bin/playwright-mcp"}},
	{PID: 200, PPID: 1, Name: "bash", Args: []string{"bash"}},
	{PID: 201, PPID: 200, Name: "grep", Args: []string{"grep", "mcp"}},
	{PID: 300, PPID: 1, Name: "node", Args: []string{"node", "/opt/github-mcp-server", "--token", "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE"}},
	{PID: 400, PPID: 1, Name: "sshd", Args: []string{"sshd"}},
	{PID: 500, PPID: 1, Name: "bash", Args: []string{"/bin/bash", "-c", "cd /x && go run ./cmd/mcpgraph scan | grep mcp"}},
}

func TestBuildSelectsInterestingProcesses(t *testing.T) {
	ev := Build(fakeTable, 999)

	got := map[int32]*model.Process{}
	for _, n := range ev.Nodes {
		got[n.Process.PID] = n.Process
	}
	for _, pid := range []int32{100, 101, 102, 103, 300} {
		if got[pid] == nil {
			t.Errorf("pid %d should be selected", pid)
		}
	}
	for _, pid := range []int32{1, 200, 201, 400, 500} {
		if got[pid] != nil {
			t.Errorf("pid %d should not be selected", pid)
		}
	}
	if got[100].AIApp != "Claude Code" {
		t.Errorf("pid 100 app = %q", got[100].AIApp)
	}
	if !got[101].MCPLike || !got[300].MCPLike || got[100].MCPLike {
		t.Error("MCPLike flags wrong")
	}
}

func TestBuildChildOfEdges(t *testing.T) {
	ev := Build(fakeTable, 999)
	ids := map[int32]string{}
	for _, n := range ev.Nodes {
		ids[n.Process.PID] = n.ID
	}
	want := map[[2]int32]bool{{101, 100}: true, {102, 101}: true, {103, 102}: true}
	for _, e := range ev.Edges {
		if e.Kind != model.EdgeChildOf {
			t.Fatalf("unexpected edge kind %s", e.Kind)
		}
		var pair [2]int32
		for pid, id := range ids {
			if id == e.From {
				pair[0] = pid
			}
			if id == e.To {
				pair[1] = pid
			}
		}
		if !want[pair] {
			t.Errorf("unexpected CHILD_OF edge %v", pair)
		}
		delete(want, pair)
	}
	if len(want) != 0 {
		t.Errorf("missing CHILD_OF edges: %v", want)
	}
}

func TestBuildRedactsCmdline(t *testing.T) {
	ev := Build(fakeTable, 999)
	for _, n := range ev.Nodes {
		if strings.Contains(n.Process.Cmdline, "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE") {
			t.Fatalf("cmdline not redacted: %s", n.Process.Cmdline)
		}
	}
}

func TestBuildSkipsSelf(t *testing.T) {
	ev := Build(fakeTable, 300)
	for _, n := range ev.Nodes {
		if n.Process.PID == 300 {
			t.Fatal("own process must not be reported")
		}
	}
}

func TestNodeIDStableAcrossScans(t *testing.T) {
	a := Build(fakeTable, 999)
	b := Build(fakeTable, 999)
	if a.Nodes[0].ID != b.Nodes[0].ID {
		t.Fatal("process IDs differ between scans of the same table")
	}
}

type failingLister struct{}

func (failingLister) List(context.Context) ([]Info, error) {
	return nil, errors.New("permission denied")
}

func TestListErrorIsWarning(t *testing.T) {
	out := make(chan model.Event, 1)
	if err := NewWithLister(failingLister{}).Collect(context.Background(), out); err != nil {
		t.Fatalf("Collect returned %v, want nil", err)
	}
	ev := <-out
	if len(ev.Warnings) != 1 {
		t.Fatalf("warnings = %v", ev.Warnings)
	}
}

func TestAppName(t *testing.T) {
	tests := []struct {
		in   Info
		want string
	}{
		{Info{Name: "claude"}, "Claude Code"},
		{Info{Name: "node", Args: []string{"node", "/usr/lib/node_modules/@anthropic-ai/claude-code/cli.js"}}, "Claude Code"},
		{Info{Name: "Claude", Exe: "/Applications/Claude.app/Contents/MacOS/Claude"}, "Claude Desktop"},
		{Info{Name: "Cursor Helper (Plugin)"}, "Cursor"},
		{Info{Name: "code"}, "VS Code"},
		{Info{Name: "node", Exe: "/home/u/.vscode-server/bin/abc/node"}, "VS Code"},
		{Info{Name: "node"}, ""},
	}
	for _, tt := range tests {
		if got := AppName(tt.in); got != tt.want {
			t.Errorf("AppName(%+v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeArgs(t *testing.T) {
	got := normalizeArgs([]string{"npm exec @playwright/mcp@latest"})
	if len(got) != 3 || got[2] != "@playwright/mcp@latest" {
		t.Errorf("normalizeArgs = %q", got)
	}
	got = normalizeArgs([]string{"/bin/my app"})
	if len(got) != 2 {
		t.Errorf("normalizeArgs = %q", got)
	}
}
