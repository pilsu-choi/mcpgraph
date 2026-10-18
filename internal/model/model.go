// Package model defines the graph types shared by every other package:
// nodes, edges, the graph itself, collector events and rule findings.
//
// The model package has no dependencies on the rest of mcpgraph, so any
// package may import it without creating an import cycle.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// NodeKind is the type of a node in the graph.
//
// Declaring a named string type (instead of using plain string) lets the
// compiler catch mistakes like passing an EdgeKind where a NodeKind is
// expected.
type NodeKind string

// Node kinds.
const (
	KindHost       NodeKind = "Host"
	KindAIApp      NodeKind = "AIApp"
	KindMCPServer  NodeKind = "MCPServer"
	KindProcess    NodeKind = "Process"
	KindCredential NodeKind = "Credential"
	KindDataSource NodeKind = "DataSource"
	KindDecision   NodeKind = "Decision"
)

// EdgeKind is the type of a directed edge in the graph.
type EdgeKind string

// Edge kinds. The comment on each one shows the direction.
const (
	EdgeConfigures EdgeKind = "CONFIGURES"  // AIApp -> MCPServer
	EdgeRunsAs     EdgeKind = "RUNS_AS"     // MCPServer -> Process
	EdgeUsesSecret EdgeKind = "USES_SECRET" // MCPServer -> Credential
	EdgeAccesses   EdgeKind = "ACCESSES"    // MCPServer -> DataSource
	EdgeChildOf    EdgeKind = "CHILD_OF"    // Process -> Process (child -> parent)
	EdgeDecidedOn  EdgeKind = "DECIDED_ON"  // Decision -> MCPServer or AIApp
)

// Transport values for MCPServer.Transport.
const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
	TransportSSE   = "sse"
)

// Credential kinds for Credential.Kind.
const (
	CredentialEnv    = "env"    // value comes from the environment, e.g. "${GITHUB_TOKEN}"
	CredentialFile   = "file"   // a path to a credential file
	CredentialInline = "inline" // a literal secret written into the config file
)

// DataSource kinds for DataSource.Kind.
const (
	DataSourceFilesystem = "filesystem"
	DataSourceDatabase   = "database"
	DataSourceSaaS       = "saas"
)

// Node is one vertex of the graph.
//
// Exactly one of the detail pointers (Host, App, Server, ...) is set,
// matching Kind. This "struct with optional pointers" is a simple way to
// model a tagged union in Go without interfaces or reflection, and it
// serialises to readable JSON because nil pointers are omitted.
type Node struct {
	ID    string   `json:"id"`
	Kind  NodeKind `json:"kind"`
	Label string   `json:"label"`

	Host       *Host       `json:"host,omitempty"`
	App        *AIApp      `json:"app,omitempty"`
	Server     *MCPServer  `json:"server,omitempty"`
	Process    *Process    `json:"process,omitempty"`
	Credential *Credential `json:"credential,omitempty"`
	DataSource *DataSource `json:"data_source,omitempty"`
	Decision   *Decision   `json:"decision,omitempty"`
}

// Host describes the machine that was scanned.
type Host struct {
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	ScannedAt time.Time `json:"scanned_at"`
}

// AIApp is an AI application whose MCP config file was found.
type AIApp struct {
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	ConfigPath string `json:"config_path"`
}

// MCPServer is one MCP server declared in an AI app config.
//
// Args and URL are already redacted. Env values are never stored, only the
// variable names.
type MCPServer struct {
	Name         string   `json:"name"`
	App          string   `json:"app"`
	Command      string   `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	EnvKeys      []string `json:"env_keys,omitempty"`
	Transport    string   `json:"transport"`
	URL          string   `json:"url,omitempty"`
	SourceConfig string   `json:"source_config"`
	// Proxied is true when the config runs the server through
	// `mcpgraph proxy -- ...`. Command and Args then describe the wrapped
	// server, not the proxy.
	Proxied bool `json:"proxied,omitempty"`
}

// Process is a running OS process. Args and Cmdline are redacted.
type Process struct {
	PID       int32     `json:"pid"`
	PPID      int32     `json:"ppid"`
	Exe       string    `json:"exe,omitempty"`
	Args      []string  `json:"args,omitempty"`
	Cmdline   string    `json:"cmdline"`
	StartedAt time.Time `json:"started_at"`
	// AIApp is set when the process is a known AI application (e.g. "Cursor").
	AIApp string `json:"ai_app,omitempty"`
	// MCPLike is true when the command line looks like an MCP server.
	MCPLike bool `json:"mcp_like,omitempty"`
}

// Credential records where a secret lives. It never contains the value.
type Credential struct {
	Kind     string `json:"kind"`
	KeyName  string `json:"key_name"`
	Location string `json:"location"`
	// Reason explains why this was classified as a credential,
	// e.g. "known token prefix ghp_".
	Reason string `json:"reason,omitempty"`
}

// DataSource is something an MCP server can read or write.
type DataSource struct {
	Kind       string `json:"kind"`
	Identifier string `json:"identifier"`
}

// Verdict is the outcome of a runtime policy decision.
type Verdict string

// Verdicts. Ask means a human must approve the action first.
const (
	VerdictAllow Verdict = "allow"
	VerdictDeny  Verdict = "deny"
	VerdictAsk   Verdict = "ask"
)

// Decision sources for Decision.Source.
const (
	SourceProxy = "proxy" // an MCP tools/call seen by `mcpgraph proxy`
	SourceHook  = "hook"  // a coding-agent tool use seen by `mcpgraph hook`
)

// Decision is one runtime policy decision. Summary is redacted before the
// Decision is created; the raw action text is never stored.
type Decision struct {
	Time   time.Time `json:"time"`
	Source string    `json:"source"`
	Agent  string    `json:"agent"`
	// App is the AI app the action came from ("Claude Code", "Cursor"),
	// when known. The graph uses Agent and App to draw DECIDED_ON edges:
	// proxy decisions point at the MCP server named Agent (in App, if
	// set), hook decisions point at the AIApp named App.
	App     string `json:"app,omitempty"`
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
	// RuleID is the policy rule that matched, or "" when no rule matched
	// and the policy default applied.
	RuleID  string  `json:"rule_id,omitempty"`
	Verdict Verdict `json:"verdict"`
	// Reason is the message shown to the agent and the user.
	Reason string `json:"reason,omitempty"`
}

// Edge is a directed relation between two nodes.
type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

// NodeID returns a stable node ID: kind + ":" + the first 12 hex characters
// of sha256(key). The same key always produces the same ID, which is what
// lets two snapshots taken at different times be diffed.
func NodeID(kind NodeKind, key string) string {
	sum := sha256.Sum256([]byte(key))
	return string(kind) + ":" + hex.EncodeToString(sum[:])[:12]
}

// Event is what a collector sends to the graph builder.
//
// Source identifies the producer (for example "config:/home/u/.cursor/mcp.json"
// or "process"). A new Event from the same Source replaces everything that
// Source contributed before, so a re-scan of one config file can add and
// remove nodes without touching the rest of the graph.
type Event struct {
	Source   string   `json:"source"`
	Nodes    []Node   `json:"nodes"`
	Edges    []Edge   `json:"edges"`
	Warnings []string `json:"warnings,omitempty"`
}

// Severity of a Finding.
type Severity string

// Severities, from most to least severe.
const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// Finding is the result of a rule matching part of the graph.
type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	NodeID   string   `json:"node_id"`
	Title    string   `json:"title"`
	// Message includes a one-line rationale for why this matters.
	Message string `json:"message"`
}
