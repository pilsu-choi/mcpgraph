package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
	"github.com/pilsu-choi/mcpgraph/policies"
)

// fakeServer is a scripted MCP server. It records every byte it receives
// and every line it sends, so tests can check the proxy changed nothing.
type fakeServer struct {
	mu       sync.Mutex
	received [][]byte
	sent     [][]byte
	out      io.Writer
}

func (f *fakeServer) run(in io.Reader, out io.WriteCloser) {
	f.out = out
	defer out.Close() // EOF on the proxy's server side: the server exited
	r := bufio.NewReader(in)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			f.mu.Lock()
			f.received = append(f.received, line)
			f.mu.Unlock()
			f.respond(line)
		}
		if err != nil {
			return
		}
	}
}

func (f *fakeServer) send(s string) {
	line := []byte(s + "\n")
	f.mu.Lock()
	f.sent = append(f.sent, line)
	f.mu.Unlock()
	_, _ = f.out.Write(line)
}

func (f *fakeServer) respond(line []byte) {
	var m map[string]json.RawMessage
	if json.Unmarshal(line, &m) != nil {
		return
	}
	var method string
	_ = json.Unmarshal(m["method"], &method)
	id, hasID := m["id"]
	if method == "" || !hasID {
		return // a notification, or the client answering a server request
	}
	reply := func(result string) { f.send(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":` + result + `}`) }
	switch method {
	case "initialize":
		// Legacy servers may send requests to the client. This one reuses
		// id 1, which the client also used: IDs are per direction.
		f.send(`{"jsonrpc":"2.0","id":1,"method":"sampling/createMessage","params":{"messages":[],"maxTokens":10}}`)
		f.send(`{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info","data":"caf\u00e9 ready"}}`)
		reply(`{"protocolVersion":"2025-11-25","capabilities":{"tools":{}},"serverInfo":{"name":"fake","version":"0"}}`)
	case "tools/list":
		reply(`{"tools":[{"name":"query","inputSchema":{"type":"object"}},{"name":"read_file","inputSchema":{"type":"object"}}]}`)
	case "tools/call":
		var params struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(m["params"], &params)
		reply(`{"content":[{"type":"text","text":"ran ` + params.Name + `"}],"isError":false}`)
	default:
		reply(`{}`)
	}
}

func (f *fakeServer) receivedLines() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.received...)
}

// memRecorder is an in-memory audit log.
type memRecorder struct {
	mu sync.Mutex
	ds []model.Decision
}

func (m *memRecorder) Append(d model.Decision) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ds = append(m.ds, d)
	return nil
}

// harness wires client <-> proxy <-> fake server with in-memory pipes.
type harness struct {
	t        *testing.T
	p        *Proxy
	server   *fakeServer
	rec      *memRecorder
	clientIn *io.PipeWriter
	lines    chan []byte // lines the client received
	got      [][]byte
	done     chan error
}

// newHarness starts a proxy with the shipped policy, unless engine is set.
func newHarness(t *testing.T, engine policy.Engine, timeout time.Duration) *harness {
	t.Helper()
	h := &harness{t: t, server: &fakeServer{}, rec: &memRecorder{}, lines: make(chan []byte, 100), done: make(chan error, 1)}
	h.p = New(Config{Server: "postgres", App: "Claude Desktop", Cwd: "/work/app", Audit: h.rec, Timeout: timeout})
	if engine == nil {
		pol, err := policy.Parse(policies.Default)
		if err != nil {
			t.Fatal(err)
		}
		e, err := policy.New(pol, policy.Options{Home: "/home/u", KnownTool: h.p.KnownTool})
		if err != nil {
			t.Fatal(err)
		}
		engine = e
	}
	h.p.SetEngine(engine)

	// io.Pipe is a synchronous in-memory pipe: a Write blocks until the
	// other side reads it, just like a real pipe with a tiny buffer.
	clientInR, clientInW := io.Pipe()
	clientOutR, clientOutW := io.Pipe()
	serverInR, serverInW := io.Pipe()
	serverOutR, serverOutW := io.Pipe()
	h.clientIn = clientInW

	go h.server.run(serverInR, serverOutW)
	go func() {
		err := h.p.Run(context.Background(), clientInR, clientOutW, serverInW, serverOutR)
		clientOutW.Close()
		h.done <- err
	}()
	go func() {
		r := bufio.NewReader(clientOutR)
		for {
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				h.lines <- line
			}
			if err != nil {
				close(h.lines)
				return
			}
		}
	}()
	return h
}

func (h *harness) send(line string) {
	h.t.Helper()
	if _, err := h.clientIn.Write([]byte(line + "\n")); err != nil {
		h.t.Fatal(err)
	}
}

// await collects client lines until a response with this id arrives and
// returns it.
func (h *harness) await(id json.RawMessage) []byte {
	h.t.Helper()
	want := idKey(id)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case line, ok := <-h.lines:
			if !ok {
				h.t.Fatalf("client output closed while waiting for id %s", want)
			}
			h.got = append(h.got, line)
			if m, ok := parseMessage(line); ok && m.isResponse() {
				if got, ok := m.id(); ok && idKey(got) == want {
					return line
				}
			}
		case <-timeout:
			h.t.Fatalf("no response for id %s within 3s; the proxy must never hang the session", want)
		}
	}
}

// finish closes the client side, waits for the proxy to stop and collects
// the remaining client lines.
func (h *harness) finish() {
	h.t.Helper()
	h.clientIn.Close()
	select {
	case err := <-h.done:
		if err != nil {
			h.t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		h.t.Fatal("proxy did not stop after the client closed stdin")
	}
	for line := range h.lines {
		h.got = append(h.got, line)
	}
}

// transcript reads a testdata/jsonrpc file, skipping comments and blanks.
func transcript(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "testdata", "jsonrpc", name))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

// replay sends every transcript line, waiting for the answer to each
// request, so the order of events is the same on every run.
func replay(h *harness, lines []string) {
	for _, l := range lines {
		h.send(l)
		m, ok := parseMessage([]byte(l))
		if !ok || m.method() == "" {
			continue
		}
		if id, ok := m.id(); ok {
			h.await(id)
		}
	}
	h.finish()
}

// expectedVerdicts per request id key, for the tools/call lines.
type verdict struct {
	verdict model.Verdict
	rule    string
}

// TestTranscriptReplay is the milestone 7 acceptance test. A recorded
// session goes through the proxy: allowed calls and all other messages
// reach the server byte for byte; denied calls never reach it and come
// back as isError results that carry the rule ID.
func TestTranscriptReplay(t *testing.T) {
	tests := []struct {
		file   string
		modern bool
		want   map[string]verdict
	}{
		{"legacy-2025-11-25.jsonl", false, map[string]verdict{
			`3`:       {model.VerdictAllow, ""},
			`"req-4"`: {model.VerdictDeny, "block-destructive-sql"},
			`6`:       {model.VerdictAsk, "ask-unknown-tool"},
			`7`:       {model.VerdictAllow, ""},
			`8`:       {model.VerdictDeny, "block-destructive-sql"}, // "Method" vs "method" bypass attempt
			`9`:       {model.VerdictDeny, "block-secret-read"},
		}},
		{"captured-sdk-1.30.0.jsonl", false, map[string]verdict{
			`2`: {model.VerdictAllow, ""},
			`3`: {model.VerdictDeny, "block-destructive-sql"},
			`4`: {model.VerdictDeny, "block-destructive-sql"},
			`5`: {model.VerdictAsk, "ask-unknown-tool"},
		}},
		{"modern-2026-07-28.jsonl", true, map[string]verdict{
			`3`: {model.VerdictAllow, ""},
			`4`: {model.VerdictDeny, "block-destructive-sql"},
			`5`: {model.VerdictAsk, "ask-git-force-push"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			lines := transcript(t, tt.file)
			h := newHarness(t, nil, 2*time.Second)
			replay(h, lines)

			// 1. What the server received: every line except denied calls
			// and the cancellation of a denied call, byte for byte.
			blocked := map[string]bool{}
			for id, v := range tt.want {
				if v.verdict != model.VerdictAllow {
					blocked[id] = true
				}
			}
			var wantServer [][]byte
			for _, l := range lines {
				m, _ := parseMessage([]byte(l))
				if id, ok := m.id(); ok && m.method() == "tools/call" && blocked[idKey(id)] {
					continue
				}
				if m.method() == "notifications/cancelled" && cancelTarget(m) != "" && blocked[cancelTarget(m)] {
					continue
				}
				wantServer = append(wantServer, []byte(l+"\n"))
			}
			gotServer := h.server.receivedLines()
			if len(gotServer) != len(wantServer) {
				t.Fatalf("server received %d lines, want %d:\n%s", len(gotServer), len(wantServer), bytes.Join(gotServer, nil))
			}
			for i := range wantServer {
				if !bytes.Equal(gotServer[i], wantServer[i]) {
					t.Errorf("server line %d changed:\n got  %q\n want %q", i, gotServer[i], wantServer[i])
				}
			}

			// 2. Every line the server sent reached the client unchanged.
			gotClient := map[string]bool{}
			for _, l := range h.got {
				gotClient[string(l)] = true
			}
			for _, l := range h.server.sent {
				if !gotClient[string(l)] {
					t.Errorf("server line did not reach the client unchanged: %q", l)
				}
			}

			// 3. Each blocked call was answered by the proxy.
			for _, l := range h.got {
				m, _ := parseMessage(l)
				id, ok := m.id()
				if !ok || !m.isResponse() || !blocked[idKey(id)] {
					continue
				}
				checkDenial(t, l, tt.want[idKey(id)], tt.modern)
				delete(blocked, idKey(id))
			}
			if len(blocked) > 0 {
				t.Errorf("no denial received for ids %v", blocked)
			}

			// 4. Every tools/call was audited, with the server and app.
			if len(h.rec.ds) != len(tt.want) {
				t.Fatalf("audited %d decisions, want %d", len(h.rec.ds), len(tt.want))
			}
			for _, d := range h.rec.ds {
				if d.Source != model.SourceProxy || d.Agent != "postgres" || d.App != "Claude Desktop" {
					t.Errorf("decision missing proxy identity: %+v", d)
				}
			}
		})
	}
}

func cancelTarget(m message) string {
	var p struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(m.fields["params"], &p) != nil || p.RequestID == nil {
		return ""
	}
	return idKey(p.RequestID)
}

// checkDenial asserts the shape of a denial: a result (not a JSON-RPC
// error) with isError, a text naming the rule, and _meta with the rule ID.
func checkDenial(t *testing.T, line []byte, want verdict, modern bool) {
	t.Helper()
	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		Error   json.RawMessage `json:"error"`
		Result  struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError    bool              `json:"isError"`
			Meta       map[string]string `json:"_meta"`
			ResultType string            `json:"resultType"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("denial is not JSON: %q", line)
	}
	r := resp.Result
	if resp.JSONRPC != "2.0" || resp.Error != nil || !r.IsError || len(r.Content) != 1 || r.Content[0].Type != "text" {
		t.Fatalf("not an isError tool result: %s", line)
	}
	if !strings.Contains(r.Content[0].Text, want.rule) || r.Meta[metaRuleID] != want.rule || r.Meta[metaVerdict] != string(want.verdict) {
		t.Errorf("denial does not carry rule %s / %s: %s", want.rule, want.verdict, line)
	}
	if want.verdict == model.VerdictAsk && !strings.Contains(r.Content[0].Text, "approval") {
		t.Errorf("ask denial must say approval is needed: %s", r.Content[0].Text)
	}
	if modern != (r.ResultType == "complete") {
		t.Errorf("resultType = %q, modern = %v", r.ResultType, modern)
	}
	if bytes.Count(line, []byte("\n")) != 1 || line[len(line)-1] != '\n' {
		t.Errorf("denial must be exactly one line: %q", line)
	}
}
