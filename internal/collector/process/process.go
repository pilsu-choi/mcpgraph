// Package process finds running AI app processes, their descendants and
// any process that looks like an MCP server.
//
// It only reads process metadata (pid, parent, command line); it never
// signals or otherwise touches a process (AGENTS.md hard rule 1).
package process

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/redact"
)

// Source is the Event.Source used for process events.
const Source = "process"

// maxDepth limits how far below an AI app process we follow children.
const maxDepth = 8

// Info is the subset of process metadata we use. Keeping our own struct
// (instead of passing gopsutil types around) lets tests build fake process
// tables without touching the real OS.
type Info struct {
	PID  int32
	PPID int32
	Name string
	Exe  string
	Args []string
	// CreateTime is milliseconds since the Unix epoch. pid+CreateTime
	// identifies a process uniquely even after the pid is reused.
	CreateTime int64
}

// Lister returns a snapshot of the process table.
type Lister interface {
	List(ctx context.Context) ([]Info, error)
}

// Collector emits one Event describing interesting processes.
type Collector struct {
	lister Lister
	self   int32
}

// New returns a Collector that reads the real process table.
func New() *Collector {
	return NewWithLister(gopsutilLister{})
}

// NewWithLister returns a Collector that reads processes from l.
func NewWithLister(l Lister) *Collector {
	return &Collector{lister: l, self: int32(os.Getpid())}
}

// Name implements collector.Collector.
func (c *Collector) Name() string { return "process" }

// Collect implements collector.Collector. Failing to list processes (for
// example in a locked-down container) is a warning, not a fatal error.
func (c *Collector) Collect(ctx context.Context, out chan<- model.Event) error {
	infos, err := c.lister.List(ctx)
	var ev model.Event
	if err != nil {
		ev = model.Event{Source: Source, Warnings: []string{fmt.Sprintf("list processes: %v", err)}}
	} else {
		ev = Build(infos, c.self)
	}
	select {
	case out <- ev:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Build selects the interesting processes from a process table and turns
// them into Process nodes and CHILD_OF edges. self is our own pid, which is
// always skipped.
//
// Interesting means: an AI app process, a descendant of one, or any
// process whose command line looks like an MCP server.
func Build(infos []Info, self int32) model.Event {
	byPID := make(map[int32]Info, len(infos))
	children := map[int32][]int32{}
	for _, in := range infos {
		byPID[in.PID] = in
		children[in.PPID] = append(children[in.PPID], in.PID)
	}

	selected := map[int32]bool{}
	for _, in := range infos {
		if in.PID == self {
			continue
		}
		if AppName(in) != "" {
			selectTree(in.PID, children, selected, 0)
		}
		if mcpLike(in) {
			selected[in.PID] = true
		}
	}
	delete(selected, self)

	ev := model.Event{Source: Source}
	for _, in := range infos {
		if !selected[in.PID] {
			continue
		}
		ev.Nodes = append(ev.Nodes, node(in))
		if parent, ok := byPID[in.PPID]; ok && selected[parent.PID] {
			ev.Edges = append(ev.Edges, model.Edge{
				From: nodeID(in),
				To:   nodeID(parent),
				Kind: model.EdgeChildOf,
			})
		}
	}
	return ev
}

// selectTree marks pid and its descendants, up to maxDepth levels.
func selectTree(pid int32, children map[int32][]int32, selected map[int32]bool, depth int) {
	if depth > maxDepth || selected[pid] {
		return
	}
	selected[pid] = true
	for _, c := range children[pid] {
		selectTree(c, children, selected, depth+1)
	}
}

func nodeID(in Info) string {
	return model.NodeID(model.KindProcess, fmt.Sprintf("%d|%d", in.PID, in.CreateTime))
}

func node(in Info) model.Node {
	args, line := redact.Cmdline(in.Args)
	name := in.Name
	if name == "" && in.Exe != "" {
		name = filepath.Base(in.Exe)
	}
	app := AppName(in)
	return model.Node{
		ID:    nodeID(in),
		Kind:  model.KindProcess,
		Label: fmt.Sprintf("%s (%d)", name, in.PID),
		Process: &model.Process{
			PID:       in.PID,
			PPID:      in.PPID,
			Exe:       in.Exe,
			Args:      args,
			Cmdline:   line,
			StartedAt: time.UnixMilli(in.CreateTime).UTC(),
			AIApp:     app,
			MCPLike:   app == "" && mcpLike(in),
		},
	}
}

// AppName returns the AI app a process belongs to, or "" if none.
func AppName(in Info) string {
	name := strings.ToLower(in.Name)
	exe := in.Exe
	joined := strings.Join(in.Args, " ")
	switch {
	case strings.Contains(exe, "Claude.app/"):
		return "Claude Desktop"
	case name == "claude" || strings.Contains(joined, "@anthropic-ai/claude-code"):
		return "Claude Code"
	case strings.HasPrefix(name, "cursor") || strings.Contains(exe, "Cursor.app/"):
		return "Cursor"
	case name == "code" || name == "code-insiders" || strings.HasPrefix(name, "code helper") ||
		strings.Contains(exe, "Visual Studio Code.app/") || strings.Contains(exe, "/.vscode-server/"):
		return "VS Code"
	}
	return ""
}

// ignoredTools are programs whose arguments often mention "mcp" without
// being an MCP server (e.g. `grep mcp`).
var ignoredTools = map[string]bool{
	"grep": true, "rg": true, "ps": true, "pgrep": true, "less": true,
	"tail": true, "cat": true, "vim": true, "nvim": true, "man": true,
}

// shells are programs whose "-c" argument is a whole script.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "fish": true}

// mcpLike reports whether a process looks like an MCP server: one of its
// non-flag arguments contains "mcp" (e.g. "@playwright/mcp@latest",
// "mcp-server-git", "/usr/local/bin/github-mcp-server").
//
// For a shell running "sh -c SCRIPT" only the first word of SCRIPT counts,
// so a long shell script that merely mentions "mcp" is not flagged.
func mcpLike(in Info) bool {
	if AppName(in) != "" || ignoredTools[strings.ToLower(in.Name)] {
		return false
	}
	args := in.Args
	if len(args) >= 3 && shells[filepath.Base(args[0])] && args[1] == "-c" {
		args = strings.Fields(args[2])
		if len(args) > 1 {
			args = args[:1]
		}
	}
	for _, a := range args {
		lower := strings.ToLower(a)
		if strings.HasPrefix(a, "-") || strings.Contains(lower, "mcpgraph") {
			continue
		}
		if strings.Contains(filepath.Base(lower), "mcp") || strings.Contains(lower, "/mcp@") {
			return true
		}
	}
	return false
}
