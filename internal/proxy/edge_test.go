package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

const drop = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users"}}}`

// hangEngine never answers and ignores ctx.
type hangEngine struct{ block chan struct{} }

func (h hangEngine) Decide(context.Context, policy.Action) (model.Decision, error) {
	<-h.block
	return model.Decision{Verdict: model.VerdictAllow}, nil
}

func TestFailClosed(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	tests := []struct {
		name   string
		engine policy.Engine
	}{
		{"engine hangs", hangEngine{block: block}},
		{"policy failed to load", FailingEngine(errors.New("policy.yaml: line 3: bad indent"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, tt.engine, 50*time.Millisecond)
			h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT 1"}}}`)
			checkDenial(t, h.await(json.RawMessage(`1`)), verdict{model.VerdictDeny, policy.FailClosedRuleID}, false)
			// Everything else keeps flowing.
			h.send(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
			if !bytes.Contains(h.await(json.RawMessage(`2`)), []byte(`"result":{}`)) {
				t.Error("ping was not answered by the server")
			}
			h.finish()
			if n := len(h.server.receivedLines()); n != 1 {
				t.Errorf("server received %d lines, want only the ping", n)
			}
		})
	}
}

func TestMalformedInput(t *testing.T) {
	h := newHarness(t, nil, time.Second)
	notJSON := `this is not json {`
	h.send(notJSON)
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`) // no name
	checkDenial(t, h.await(json.RawMessage(`1`)), verdict{model.VerdictDeny, policy.FailClosedRuleID}, false)
	h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":"DROP TABLE users"}`) // params not an object
	checkDenial(t, h.await(json.RawMessage(`2`)), verdict{model.VerdictDeny, policy.FailClosedRuleID}, false)
	h.send(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE x"}}}`) // notification
	h.send(`{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	h.await(json.RawMessage(`3`))
	h.finish()

	got := h.server.receivedLines()
	if len(got) != 2 || string(got[0]) != notJSON+"\n" || !strings.Contains(string(got[1]), `"ping"`) {
		t.Fatalf("server received %q; want only the non-JSON line and the ping", got)
	}
	for _, l := range h.got {
		if bytes.Contains(l, []byte("DROP TABLE x")) || (bytes.Contains(l, []byte(`"id":null`))) {
			t.Errorf("a tools/call notification must get no reply: %q", l)
		}
	}
}

func TestLongMessageIsForwardedUnchanged(t *testing.T) {
	h := newHarness(t, nil, time.Second)
	big := strings.Repeat("a", 2<<20) // 2 MiB, far past bufio.Scanner's 64 KiB default
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT '` + big + `'"}}}`
	h.send(line)
	h.await(json.RawMessage(`1`))
	h.finish()
	got := h.server.receivedLines()
	if len(got) != 1 || !bytes.Equal(got[0], []byte(line+"\n")) {
		t.Fatal("long message was changed or not forwarded")
	}
}

func TestCRLFIsForwardedUnchanged(t *testing.T) {
	h := newHarness(t, nil, time.Second)
	if _, err := h.clientIn.Write([]byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\r\n")); err != nil {
		t.Fatal(err)
	}
	h.await(json.RawMessage(`1`))
	if _, err := h.clientIn.Write([]byte(drop[:len(drop)-1] + "}\r\n")); err != nil {
		t.Fatal(err)
	}
	checkDenial(t, h.await(json.RawMessage(`1`)), verdict{model.VerdictDeny, "block-destructive-sql"}, false)
	h.finish()
	if got := h.server.receivedLines(); len(got) != 1 || !bytes.HasSuffix(got[0], []byte("\r\n")) {
		t.Fatalf("server received %q", got)
	}
}

func TestBatches(t *testing.T) {
	h := newHarness(t, nil, time.Second)
	safe := `[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"}]`
	h.send(safe)
	// The fake server does not answer batches; a ping afterwards shows
	// the batch has been forwarded.
	h.send(`{"jsonrpc":"2.0","id":9,"method":"ping"}`)
	h.await(json.RawMessage(`9`))
	bad := `[{"jsonrpc":"2.0","id":2,"method":"ping"},` + strings.Replace(drop, `"id":1`, `"id":3`, 1) + `]`
	h.send(bad)
	reply := waitLine(t, h, func(l []byte) bool { return bytes.HasPrefix(l, []byte("[")) })
	h.finish()

	got := h.server.receivedLines()
	if len(got) != 2 || string(got[0]) != safe+"\n" {
		t.Fatalf("server received %q; the batch with DROP must not be forwarded", got)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(reply, &items); err != nil || len(items) != 2 {
		t.Fatalf("batch reply %q", reply)
	}
	if _, ok := items[0]["error"]; !ok {
		t.Errorf("ping in a held-back batch should get an error: %s", items[0]["error"])
	}
	if !bytes.Contains(items[1]["result"], []byte(`"isError":true`)) {
		t.Errorf("tools/call in the batch should get an isError result: %s", items[1]["result"])
	}
}

func waitLine(t *testing.T, h *harness, match func([]byte) bool) []byte {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case l := <-h.lines:
			h.got = append(h.got, l)
			if match(l) {
				return l
			}
		case <-timeout:
			t.Fatal("timed out")
		}
	}
}

func TestUnknownToolNeedsAnInventory(t *testing.T) {
	h := newHarness(t, nil, time.Second)
	// No tools/list yet: the inventory is unknown, so the call is judged
	// only by the other rules and allowed.
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"brand_new","arguments":{"q":"hello"}}}`)
	if l := h.await(json.RawMessage(`1`)); !bytes.Contains(l, []byte("ran brand_new")) {
		t.Fatalf("call before tools/list should reach the server: %s", l)
	}
	h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	h.await(json.RawMessage(`2`))
	h.send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"brand_new","arguments":{"q":"hello"}}}`)
	checkDenial(t, h.await(json.RawMessage(`3`)), verdict{model.VerdictAsk, "ask-unknown-tool"}, false)
	h.finish()
}

func TestServerExitDoesNotHang(t *testing.T) {
	p := New(Config{Server: "s", Timeout: time.Second})
	p.SetEngine(FailingEngine(errors.New("unused")))
	clientInR, _ := io.Pipe() // the client never closes stdin
	serverInR, serverInW := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, serverInR) }()
	done := make(chan error, 1)
	go func() {
		done <- p.Run(context.Background(), clientInR, io.Discard, serverInW, strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/message","params":{}}`+"\n"))
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the server's output ended")
	}
}

func TestNoSecretInDenialOrAudit(t *testing.T) {
	h := newHarness(t, nil, time.Second)
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users","dsn":"postgresql://app:FakePassw0rdXyz@db/prod","token":"ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234"}}}`)
	h.await(json.RawMessage(`1`))
	h.finish()
	var all bytes.Buffer
	for _, l := range h.got {
		all.Write(l)
	}
	for _, d := range h.rec.ds {
		b, _ := json.Marshal(d)
		all.Write(b)
	}
	for _, s := range testutil.FakeSecrets {
		if bytes.Contains(all.Bytes(), []byte(s)) {
			t.Errorf("secret %q leaked into a denial or decision", s)
		}
	}
}

// --- RunCommand with a real child process -------------------------------

// TestMain lets the test binary act as an MCP server: when the env var is
// set, it runs the fake server on stdin/stdout instead of the tests. This
// is the standard way to test os/exec code without a separate binary.
func TestMain(m *testing.M) {
	if os.Getenv("MCPGRAPH_FAKE_SERVER") == "1" {
		fmt.Fprintln(os.Stderr, "fake server: starting") // stderr must pass through
		f := &fakeServer{}
		f.run(os.Stdin, nopCloser{os.Stdout})
		os.Exit(3)
	}
	os.Exit(m.Run())
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func TestRunCommand(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCPGRAPH_FAKE_SERVER", "1")
	p := New(Config{Server: "fake", Timeout: time.Second})
	p.SetEngine(FailingEngine(errors.New("deny everything")))

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" + drop + "\n")
	var out, errOut bytes.Buffer
	code, err := p.RunCommand(context.Background(), []string{exe, "-test.run=^$"}, in, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Errorf("exit code %d, want the child's 3", code)
	}
	if !strings.Contains(errOut.String(), "fake server: starting") {
		t.Errorf("child stderr not passed through: %q", errOut.String())
	}
	var ids []string
	sc := bufio.NewScanner(&out)
	for sc.Scan() {
		m, _ := parseMessage(sc.Bytes())
		id, _ := m.id()
		ids = append(ids, string(id))
		if strings.Contains(sc.Text(), `"isError":true`) && string(id) != "1" {
			t.Errorf("wrong id on denial: %s", sc.Text())
		}
	}
	if len(ids) != 2 {
		t.Fatalf("client got %d lines (%q), want the ping reply and the denial", len(ids), out.String())
	}
}

func TestRunCommandMissingBinary(t *testing.T) {
	p := New(Config{})
	if _, err := p.RunCommand(context.Background(), []string{"/nonexistent/mcp-server"}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("want an error for a missing server binary")
	}
	if _, err := p.RunCommand(context.Background(), nil, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("want an error for an empty command")
	}
}
