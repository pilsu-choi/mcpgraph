package policy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/redact"
)

// Action is what an agent is about to do, normalized from either
// interception path.
type Action struct {
	Source string // model.SourceProxy or model.SourceHook
	Agent  string // e.g. "claude-code", or the MCP server name
	Tool   string // MCP tool name, or "Bash" etc. for hooks
	// Args are the raw tool arguments. They may hold secrets: never log
	// them; the Decision only carries a redacted summary.
	Args map[string]any
	// Text is the command or query the matchers inspect. When empty, the
	// string values of Args are inspected instead.
	Text string
	// Cwd is the agent's working directory (the project), when known. It
	// is not in the AGENTS.md sketch of Action; recursive_delete needs it
	// to tell "inside the project" from "outside".
	Cwd string
}

// Engine is the one interface both interception paths depend on.
type Engine interface {
	Decide(ctx context.Context, a Action) (model.Decision, error)
}

// Options configure a RuleEngine.
type Options struct {
	// Home expands "~" in path globs and commands.
	Home string
	// KnownTool reports whether the last scan saw this tool on this MCP
	// server. When nil, the tool inventory is unknown and unknown_tool
	// matchers never fire.
	KnownTool func(server, tool string) bool
	// Now returns the decision time; tests replace it. Default time.Now.
	Now func() time.Time
}

// RuleEngine evaluates a compiled Policy. It is safe for concurrent use:
// after New returns, nothing in it is modified.
type RuleEngine struct {
	rules []compiledRule
	def   model.Verdict
	opts  Options
}

// Compile-time check that *RuleEngine satisfies Engine. If a method is
// missing or has the wrong signature, this line fails to build.
var _ Engine = (*RuleEngine)(nil)

// New validates and compiles a policy.
func New(p *Policy, opts Options) (*RuleEngine, error) {
	rules, err := compile(p, opts.Home)
	if err != nil {
		return nil, err
	}
	def := p.Default
	if def == "" {
		def = model.VerdictAllow
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &RuleEngine{rules: rules, def: def, opts: opts}, nil
}

// maxSummary caps the redacted action text stored on a Decision.
const maxSummary = 200

// Decide returns the verdict of the first matching rule, or the policy
// default. It returns an error only when ctx is already done; callers on
// the tools/call path use Judge, which turns any error into a denial.
func (e *RuleEngine) Decide(ctx context.Context, a Action) (model.Decision, error) {
	if err := ctx.Err(); err != nil {
		return model.Decision{}, err
	}
	in := newInput(a, e.opts.Home)
	for i := range e.rules {
		r := &e.rules[i]
		if r.applies(a) && r.matches(in, e.opts) {
			return e.decision(a, in.text, r.ID, r.Verdict, r.Reason), nil
		}
	}
	return e.decision(a, in.text, "", e.def, ""), nil
}

func (e *RuleEngine) decision(a Action, text, ruleID string, v model.Verdict, reason string) model.Decision {
	return model.Decision{
		Time:    e.opts.Now().UTC(),
		Source:  a.Source,
		Agent:   a.Agent,
		Tool:    a.Tool,
		Summary: Summary(text),
		RuleID:  ruleID,
		Verdict: v,
		Reason:  reason,
	}
}

// Summary redacts and shortens action text for a Decision. Redaction runs
// before truncation, so a secret cut in half is still masked.
func Summary(text string) string {
	s := redact.Text(text)
	if r := []rune(s); len(r) > maxSummary {
		s = string(r[:maxSummary]) + "…"
	}
	return s
}

// input is an Action prepared once for all rules: the text, its shell
// segments, the SQL classes found and the path-like words.
type input struct {
	action Action
	text   string
	segs   []segment
	sql    map[string]bool
	paths  []string // resolved, cleaned
}

func newInput(a Action, home string) *input {
	argStrings := stringValues(a.Args)
	text := a.Text
	if text == "" {
		text = strings.Join(argStrings, "\n")
	}
	segs := splitShell(text)
	in := &input{action: a, text: text, segs: segs, sql: classifySQL(text)}
	for _, w := range pathCandidates(segs, argStrings) {
		in.paths = append(in.paths, resolvePath(w, home, a.Cwd))
	}
	return in
}

// applies checks the rule's `when` filters.
func (r *compiledRule) applies(a Action) bool {
	if len(r.sources) > 0 && !r.sources[a.Source] {
		return false
	}
	if len(r.tools) == 0 {
		return true
	}
	for _, re := range r.tools {
		if re.MatchString(a.Tool) {
			return true
		}
	}
	return false
}

// matches reports whether any matcher of the rule fires.
func (r *compiledRule) matches(in *input, opts Options) bool {
	for _, class := range r.Match.SQL {
		if in.sql[class] {
			return true
		}
	}
	for _, re := range r.text {
		if re.MatchString(in.text) {
			return true
		}
	}
	for _, s := range in.segs {
		for _, re := range r.command {
			if re.MatchString(s.commandLine()) {
				return true
			}
		}
		if r.Match.RecursiveDelete == RecursiveDeleteOutsideProject && recursiveDeleteOutside(s, opts.Home, in.action.Cwd) {
			return true
		}
	}
	if len(r.paths) > 0 && r.matchPaths(in.paths) {
		return true
	}
	if r.Match.UnknownTool && opts.KnownTool != nil && in.action.Source == model.SourceProxy {
		return !opts.KnownTool(in.action.Agent, in.action.Tool)
	}
	return false
}

func (r *compiledRule) matchPaths(paths []string) bool {
	for _, p := range paths {
		if anyGlob(r.paths, p) && !anyGlob(r.pathsExcept, p) {
			return true
		}
	}
	return false
}

func anyGlob(globs []globPattern, p string) bool {
	for _, g := range globs {
		if g.match(p) {
			return true
		}
	}
	return false
}

// stringValues collects every string inside args, depth first, with map
// keys sorted so the result does not depend on Go's random map order.
func stringValues(v any) []string {
	var out []string
	switch t := v.(type) {
	case string:
		out = append(out, t)
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, stringValues(t[k])...)
		}
	case []any:
		for _, x := range t {
			out = append(out, stringValues(x)...)
		}
	}
	return out
}

// FailClosedRuleID is the rule ID on decisions made because the engine
// failed or was too slow.
const FailClosedRuleID = "mcpgraph-fail-closed"

// Judge runs e.Decide with a timeout and never fails: if the engine returns
// an error, panics or does not answer in time, the action is denied (hard
// rule 4: fail closed on tools/call). The caller is never blocked for
// longer than timeout.
func Judge(ctx context.Context, e Engine, a Action, timeout time.Duration) model.Decision {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	// defer runs when Judge returns, so the timer behind the context is
	// released even on the fast path.
	defer cancel()

	type result struct {
		d   model.Decision
		err error
	}
	// Buffered with room for one value: if we stop waiting after a
	// timeout, the goroutine can still send its late result and exit
	// instead of blocking forever.
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				ch <- result{err: fmt.Errorf("policy engine panic: %v", p)}
			}
		}()
		d, err := e.Decide(ctx, a)
		ch <- result{d: d, err: err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return failClosed(a, r.err)
		}
		return r.d
	case <-ctx.Done():
		return failClosed(a, ctx.Err())
	}
}

func failClosed(a Action, err error) model.Decision {
	text := a.Text
	if text == "" {
		text = strings.Join(stringValues(a.Args), "\n")
	}
	return model.Decision{
		Time:    time.Now().UTC(),
		Source:  a.Source,
		Agent:   a.Agent,
		Tool:    a.Tool,
		Summary: Summary(text),
		RuleID:  FailClosedRuleID,
		Verdict: model.VerdictDeny,
		Reason:  fmt.Sprintf("mcpgraph could not evaluate the policy (%v), so the call was denied (fail closed)", err),
	}
}
