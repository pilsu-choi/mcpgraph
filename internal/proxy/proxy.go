// Package proxy is a transparent stdio MCP proxy that judges tools/call
// requests with the policy engine.
//
//	AI app (client) ──stdin──▶ proxy ──▶ MCP server
//	                ◀─stdout── proxy ◀──
//
// Everything except tools/call is copied byte for byte in both directions.
// A tools/call is judged first: allowed calls are forwarded unchanged,
// denied (and ask) calls are answered by the proxy and never reach the
// server. The proxy works with both MCP revisions in use (2025-11-25 with
// the initialize handshake and server-to-client requests, and the
// stateless 2026-07-28) because it only needs what both share: newline
// framing, JSON-RPC ids and the tools/call shape.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
)

// Recorder stores decisions. *audit.Log satisfies it.
type Recorder interface {
	Append(d model.Decision) error
}

// Config configures one proxy session (one MCP server).
type Config struct {
	Engine policy.Engine
	// Audit receives every decision; nil disables auditing.
	Audit Recorder
	// Server is the MCP server's name in the AI app config; it becomes
	// the Decision's Agent, which the graph links to the MCPServer node.
	Server string
	// App is the AI app that runs the server, e.g. "Cursor".
	App string
	// Cwd is the working directory passed to policy actions.
	Cwd string
	// Timeout bounds one policy decision (hard rule 4). Default 2s.
	Timeout time.Duration
	// Logger receives diagnostics; it must write to stderr, never stdout,
	// because stdout is the MCP channel. Default: slog.Default().
	Logger *slog.Logger
}

// Proxy carries the state shared by the two directions.
type Proxy struct {
	cfg Config

	mu sync.Mutex
	// inflight maps client request IDs to their method, for requests the
	// client sent to the server. It lets the server-to-client side know
	// which response answers a tools/list. Server-to-client requests
	// (legacy revision) have their own ID space and are not tracked.
	inflight map[string]string
	// answered holds IDs of tools/call requests the proxy answered
	// itself. The server never saw them, so a later
	// notifications/cancelled for one of them is dropped, not forwarded.
	answered map[string]bool
	// tools is the inventory from tools/list responses; nil until the
	// first one arrives.
	tools map[string]bool
}

// New returns a Proxy. Use KnownTool as the policy engine's KnownTool
// option so ask-unknown-tool sees this session's inventory.
func New(cfg Config) *Proxy {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Proxy{cfg: cfg, inflight: map[string]string{}, answered: map[string]bool{}}
}

// SetAudit attaches the audit log.
func (p *Proxy) SetAudit(r Recorder) { p.cfg.Audit = r }

// SetEngine sets the policy engine after New. It exists because the engine
// needs p.KnownTool, so it can only be built once the Proxy exists.
func (p *Proxy) SetEngine(e policy.Engine) { p.cfg.Engine = e }

// KnownTool reports whether the server listed the tool in a tools/list
// response during this session. Until the first tools/list response the
// inventory is unknown and every tool counts as known, so
// ask-unknown-tool does not fire on clients that call before listing.
func (p *Proxy) KnownTool(_ /* server */, tool string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tools == nil || p.tools[tool]
}

// Run proxies until the server's output ends (the server exited or closed
// stdout), then returns. When the client's input ends, the server's stdin
// is closed so the server can exit, as the MCP stdio shutdown sequence
// expects.
//
// One goroutine runs each direction. Run waits only for the
// server-to-client goroutine: the client-to-server one may be blocked in
// a read on the client's stdin, which cannot be interrupted, and it ends
// when the process exits.
func (p *Proxy) Run(ctx context.Context, clientIn io.Reader, clientOut io.Writer, serverIn io.WriteCloser, serverOut io.Reader) error {
	out := &syncWriter{w: clientOut}
	go func() {
		if err := p.clientToServer(ctx, clientIn, out, serverIn); err != nil {
			p.cfg.Logger.Warn("mcpgraph proxy: client to server", "err", err)
		}
	}()
	return p.serverToClient(serverOut, out)
}

// clientToServer reads client messages, judges tools/call and forwards the
// rest unchanged.
func (p *Proxy) clientToServer(ctx context.Context, clientIn io.Reader, out *syncWriter, serverIn io.WriteCloser) error {
	// Closing the server's stdin on return is how the server learns the
	// client has gone (MCP stdio shutdown).
	defer serverIn.Close()
	r := newLineReader(clientIn)
	for {
		line, err := r.next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		forward, reply := p.inspectClient(ctx, line)
		if reply != nil {
			if err := out.writeLine(reply); err != nil {
				return fmt.Errorf("write to client: %w", err)
			}
		}
		if forward {
			if _, err := serverIn.Write(line); err != nil {
				return fmt.Errorf("write to server: %w", err)
			}
		}
	}
}

// inspectClient decides what to do with one client line: forward it
// (true) and/or send reply to the client.
func (p *Proxy) inspectClient(ctx context.Context, line []byte) (forward bool, reply []byte) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return p.inspectBatch(ctx, trimmed)
	}
	msg, ok := parseMessage(line)
	if !ok {
		// Not a JSON object: not something we judge. The server will
		// answer with a parse error.
		return true, nil
	}
	method := msg.method()
	id, hasID := msg.id()
	switch {
	case method == "tools/call":
		return p.handleToolCall(ctx, msg, id, hasID)
	case method == "notifications/cancelled" && p.cancelsAnswered(msg):
		return false, nil
	case method != "" && hasID:
		p.mu.Lock()
		p.inflight[idKey(id)] = method
		p.mu.Unlock()
	}
	return true, nil
}

// handleToolCall judges one tools/call request.
func (p *Proxy) handleToolCall(ctx context.Context, msg message, id json.RawMessage, hasID bool) (bool, []byte) {
	d := p.judge(ctx, msg)
	p.record(d)
	if d.Verdict == model.VerdictAllow {
		if hasID {
			p.mu.Lock()
			p.inflight[idKey(id)] = "tools/call"
			p.mu.Unlock()
		}
		return true, nil
	}
	if !hasID {
		// A tools/call without an id is a notification and gets no
		// reply; it is still not forwarded.
		return false, nil
	}
	p.mu.Lock()
	p.answered[idKey(id)] = true
	p.mu.Unlock()
	return false, denialResponse(id, d, toolCallIsModern(msg))
}

func toolCallIsModern(msg message) bool {
	tc, err := parseToolCall(msg)
	return err == nil && tc.modern
}

// judge turns a tools/call into a policy decision. Unreadable params are
// denied: the proxy cannot judge what it cannot read (fail closed).
func (p *Proxy) judge(ctx context.Context, msg message) model.Decision {
	tc, err := parseToolCall(msg)
	if err != nil {
		return model.Decision{
			Time: time.Now().UTC(), Source: model.SourceProxy, Agent: p.cfg.Server, App: p.cfg.App, Tool: "?",
			RuleID: policy.FailClosedRuleID, Verdict: model.VerdictDeny,
			Reason: "mcpgraph could not read this tools/call (" + err.Error() + "), so it was denied (fail closed).",
		}
	}
	a := policy.Action{
		Source: model.SourceProxy, Agent: p.cfg.Server, App: p.cfg.App, Tool: tc.name, Args: tc.args, Cwd: p.cfg.Cwd,
	}
	if p.cfg.Engine == nil {
		return policy.Judge(ctx, failingEngine{errors.New("no policy engine configured")}, a, p.cfg.Timeout)
	}
	return policy.Judge(ctx, p.cfg.Engine, a, p.cfg.Timeout)
}

// inspectBatch handles a JSON array. Batches were removed from MCP in
// 2025-06-18 (research §A2), but an old client could still send one. A
// batch without tools/call is forwarded unchanged. A batch whose tools/call
// requests are all allowed is forwarded too. Otherwise none of it is
// forwarded: splitting a batch would change the server's view of it, so
// the whole batch is answered with denials (fail closed).
func (p *Proxy) inspectBatch(ctx context.Context, line []byte) (bool, []byte) {
	var items []json.RawMessage
	if err := json.Unmarshal(line, &items); err != nil {
		return true, nil
	}
	type pending struct {
		id     json.RawMessage
		d      model.Decision
		isCall bool
	}
	var reqs []pending
	blocked := false
	for _, raw := range items {
		msg, ok := parseMessage(raw)
		if !ok {
			continue
		}
		id, hasID := msg.id()
		if msg.method() == "tools/call" {
			d := p.judge(ctx, msg)
			p.record(d)
			if d.Verdict != model.VerdictAllow {
				blocked = true
			}
			if hasID {
				reqs = append(reqs, pending{id: id, d: d, isCall: true})
			}
		} else if msg.method() != "" && hasID {
			reqs = append(reqs, pending{id: id})
		}
	}
	if !blocked {
		return true, nil
	}
	var replies []json.RawMessage
	for _, r := range reqs {
		if r.isCall && r.d.Verdict != model.VerdictAllow {
			replies = append(replies, bytes.TrimSpace(denialResponse(r.id, r.d, false)))
			continue
		}
		// Not run because the batch was held back.
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": r.id, "error": map[string]any{
			"code": -32603, "message": "mcpgraph: batch not forwarded because it contained a denied tools/call",
		}})
		replies = append(replies, b)
	}
	if len(replies) == 0 {
		return false, nil
	}
	b, _ := json.Marshal(replies)
	return false, append(b, '\n')
}

// cancelsAnswered reports whether a notifications/cancelled refers to a
// request the proxy answered itself.
func (p *Proxy) cancelsAnswered(msg message) bool {
	var params struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(msg.fields["params"], &params) != nil || params.RequestID == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.answered[idKey(params.RequestID)]
}

// serverToClient copies server output to the client unchanged, learning
// the tool inventory from tools/list responses on the way.
func (p *Proxy) serverToClient(serverOut io.Reader, out *syncWriter) error {
	r := newLineReader(serverOut)
	for {
		line, err := r.next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read from server: %w", err)
		}
		p.inspectServer(line)
		if err := out.writeLine(line); err != nil {
			return fmt.Errorf("write to client: %w", err)
		}
	}
}

// inspectServer looks at a server line without changing it.
func (p *Proxy) inspectServer(line []byte) {
	msg, ok := parseMessage(line)
	if !ok || !msg.isResponse() {
		return
	}
	id, ok := msg.id()
	if !ok {
		return
	}
	key := idKey(id)
	p.mu.Lock()
	method := p.inflight[key]
	delete(p.inflight, key)
	p.mu.Unlock()
	if method == "tools/list" {
		p.learnTools(msg)
	}
}

// learnTools adds the names in a tools/list result to the inventory.
// Paginated lists arrive as several responses; the inventory is their
// union.
func (p *Proxy) learnTools(msg message) {
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(msg.fields["result"], &result) != nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tools == nil {
		p.tools = map[string]bool{}
	}
	for _, t := range result.Tools {
		p.tools[t.Name] = true
	}
}

// record appends a decision to the audit log. A failing audit log does
// not change the decision; it is reported on stderr.
func (p *Proxy) record(d model.Decision) {
	if p.cfg.Audit == nil {
		return
	}
	if err := p.cfg.Audit.Append(d); err != nil {
		p.cfg.Logger.Warn("mcpgraph proxy: audit log", "err", err)
	}
}

// failingEngine makes every decision fail, so Judge denies (fail closed).
type failingEngine struct{ err error }

func (f failingEngine) Decide(context.Context, policy.Action) (model.Decision, error) {
	return model.Decision{}, f.err
}

// FailingEngine returns an engine whose every decision fails. The CLI uses
// it when the policy cannot be loaded: the proxy still starts and passes
// everything else through, but denies every tools/call.
func FailingEngine(err error) policy.Engine { return failingEngine{err: err} }

// Cwd returns the working directory for policy actions, or "".
func Cwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}
