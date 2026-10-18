// Package policy decides whether an action an AI agent is about to take is
// allowed, denied or needs a human to approve it.
//
// Both runtime interception paths (the MCP proxy and the coding-agent hook)
// turn what they see into an Action and call the same Engine, so a rule is
// written once in YAML and applies everywhere.
package policy

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// Policy is the YAML document, e.g. policies/default.yaml.
type Policy struct {
	Version int `yaml:"version"`
	// Default is the verdict when no rule matches. Empty means allow.
	Default model.Verdict `yaml:"default"`
	Rules   []Rule        `yaml:"rules"`
}

// Rule is one policy rule. Rules are evaluated in order; the first match
// wins.
type Rule struct {
	ID      string        `yaml:"id"`
	Verdict model.Verdict `yaml:"verdict"`
	// Reason is shown to the agent (deny) or the user (ask).
	Reason string `yaml:"reason"`
	When   When   `yaml:"when"`
	Match  Match  `yaml:"match"`
}

// When narrows which actions a rule looks at. Every field that is set must
// hold.
type When struct {
	// Sources limits the rule to "proxy" and/or "hook" actions.
	Sources []string `yaml:"sources"`
	// Tools are regexes; the action's tool name must match one of them.
	Tools []string `yaml:"tools"`
}

// Match lists the matchers of a rule. The rule matches when ANY of the
// matchers that are set fires.
type Match struct {
	// SQL lists statement classes: drop_table, drop_database, truncate,
	// delete_without_where.
	SQL []string `yaml:"sql"`
	// Command regexes are matched against each simple command of the
	// action text ("git push --force"), with the command word reduced to
	// its base name. They never span two commands joined by && or ;.
	Command []string `yaml:"command"`
	// Text regexes are matched against the whole action text.
	Text []string `yaml:"text"`
	// RecursiveDelete set to "outside_project" matches rm -r whose target
	// is not strictly inside the action's working directory.
	RecursiveDelete string `yaml:"recursive_delete"`
	// Paths are globs ("~/.ssh/**", "**/.env") matched against every
	// path-like word in the text and the arguments.
	Paths []string `yaml:"paths"`
	// PathsExcept are globs that override Paths ("**/.env.example").
	PathsExcept []string `yaml:"paths_except"`
	// UnknownTool matches MCP tools that the last scan did not see.
	UnknownTool bool `yaml:"unknown_tool"`
}

// RecursiveDeleteOutsideProject is the only RecursiveDelete mode.
const RecursiveDeleteOutsideProject = "outside_project"

// LoadFile reads and parses a policy file.
func LoadFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy %s: %w", path, err)
	}
	p, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("policy %s: %w", path, err)
	}
	return p, nil
}

// Parse decodes a policy document. Unknown keys are an error, so a typo
// such as "verdikt:" cannot silently disable a rule.
func Parse(data []byte) (*Policy, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var p Policy
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return &p, nil
}

// compiledRule is a Rule with its regexes and globs compiled. Compiling
// once at load time keeps each decision fast.
type compiledRule struct {
	Rule
	sources     map[string]bool
	tools       []*regexp.Regexp
	command     []*regexp.Regexp
	text        []*regexp.Regexp
	paths       []globPattern
	pathsExcept []globPattern
}

// compile validates the policy and compiles every rule. home expands "~"
// in path globs.
func compile(p *Policy, home string) ([]compiledRule, error) {
	if p.Version != 1 {
		return nil, fmt.Errorf("unsupported policy version %d (want 1)", p.Version)
	}
	if p.Default != "" && !validVerdict(p.Default) {
		return nil, fmt.Errorf("default: unknown verdict %q", p.Default)
	}
	seen := map[string]bool{}
	var out []compiledRule
	for i, r := range p.Rules {
		c, err := compileRule(r, home)
		if err != nil {
			return nil, fmt.Errorf("rule %d (%s): %w", i+1, r.ID, err)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("rule %d: duplicate id %q", i+1, r.ID)
		}
		seen[r.ID] = true
		out = append(out, c)
	}
	return out, nil
}

func compileRule(r Rule, home string) (compiledRule, error) {
	c := compiledRule{Rule: r, sources: map[string]bool{}}
	if r.ID == "" {
		return c, errors.New("missing id")
	}
	if !validVerdict(r.Verdict) {
		return c, fmt.Errorf("unknown verdict %q (want allow, deny or ask)", r.Verdict)
	}
	for _, s := range r.When.Sources {
		if s != model.SourceProxy && s != model.SourceHook {
			return c, fmt.Errorf("unknown source %q (want proxy or hook)", s)
		}
		c.sources[s] = true
	}
	for _, s := range r.Match.SQL {
		if !sqlClasses[s] {
			return c, fmt.Errorf("unknown sql class %q", s)
		}
	}
	if r.Match.RecursiveDelete != "" && r.Match.RecursiveDelete != RecursiveDeleteOutsideProject {
		return c, fmt.Errorf("unknown recursive_delete mode %q (want %s)", r.Match.RecursiveDelete, RecursiveDeleteOutsideProject)
	}
	var err error
	if c.tools, err = compileRegexes(r.When.Tools, true); err != nil {
		return c, fmt.Errorf("when.tools: %w", err)
	}
	if c.command, err = compileRegexes(r.Match.Command, false); err != nil {
		return c, fmt.Errorf("match.command: %w", err)
	}
	if c.text, err = compileRegexes(r.Match.Text, false); err != nil {
		return c, fmt.Errorf("match.text: %w", err)
	}
	if c.paths, err = compileGlobs(r.Match.Paths, home); err != nil {
		return c, fmt.Errorf("match.paths: %w", err)
	}
	if c.pathsExcept, err = compileGlobs(r.Match.PathsExcept, home); err != nil {
		return c, fmt.Errorf("match.paths_except: %w", err)
	}
	if !c.hasMatcher() {
		return c, errors.New("match has no matcher, so the rule would never fire")
	}
	return c, nil
}

func (c compiledRule) hasMatcher() bool {
	m := c.Match
	return len(m.SQL) > 0 || len(c.command) > 0 || len(c.text) > 0 ||
		m.RecursiveDelete != "" || len(c.paths) > 0 || m.UnknownTool
}

func validVerdict(v model.Verdict) bool {
	return v == model.VerdictAllow || v == model.VerdictDeny || v == model.VerdictAsk
}

// compileRegexes compiles case-insensitive regexes. anchored wraps each in
// ^(?:...)$, which is what tool-name filters want.
func compileRegexes(exprs []string, anchored bool) ([]*regexp.Regexp, error) {
	var out []*regexp.Regexp
	for _, e := range exprs {
		src := e
		if anchored {
			src = "^(?:" + e + ")$"
		}
		re, err := regexp.Compile("(?i)" + src)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

func compileGlobs(patterns []string, home string) ([]globPattern, error) {
	var out []globPattern
	for _, p := range patterns {
		g, err := compileGlob(p, home)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", p, err)
		}
		out = append(out, g)
	}
	return out, nil
}
