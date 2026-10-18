package central

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/sink"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// agent creates a data dir with an audit log of policy-shaped decisions
// (one summary carries a fake secret) and a sink pointing at url.
func agent(t *testing.T, url, token, host string, verdicts ...model.Verdict) string {
	t.Helper()
	dir := t.TempDir()
	l, err := audit.Open(filepath.Join(dir, audit.FileName))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for i, v := range verdicts {
		d := model.Decision{
			Time: time.Date(2026, 10, 18, 8, i, 0, 0, time.UTC), Source: model.SourceProxy, Agent: host + "-pg",
			Tool: "query", Summary: "psql password=" + testutil.FakeSecrets[1] + " n=" + string(rune('0'+i)), Verdict: v,
		}
		if v == model.VerdictDeny {
			d.RuleID = "block-destructive-sql"
			d.Reason = "token " + testutil.FakeSecrets[0]
		}
		if err := l.Append(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := sink.SaveConfig(dir, sink.Config{URL: url, Token: token, Host: host}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newCollector(t *testing.T, dir, token string) (*Store, *httptest.Server) {
	t.Helper()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(store, token))
	t.Cleanup(func() { srv.Close(); store.Close() })
	return store, srv
}

func get(t *testing.T, url, token string, v any) string {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if v != nil {
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatalf("decode %s: %v", b, err)
		}
	}
	return string(b)
}

// TestTwoAgentsReportToOneCollector is the milestone's acceptance test.
func TestTwoAgentsReportToOneCollector(t *testing.T) {
	colDir := t.TempDir()
	var posts atomic.Int32
	_, srv := newCollector(t, colDir, "tok")
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer counting.Close()

	a := agent(t, counting.URL, "tok", "laptop-a", model.VerdictAllow, model.VerdictDeny, model.VerdictAllow)
	b := agent(t, counting.URL, "tok", "laptop-b", model.VerdictDeny, model.VerdictAsk)
	ctx := context.Background()
	for _, dir := range []string{a, b} {
		if err := sink.Ship(ctx, dir); err != nil {
			t.Fatal(err)
		}
	}

	var hosts []HostSummary
	body := get(t, srv.URL+"/v1/hosts", "tok", &hosts)
	if len(hosts) != 2 || hosts[0].Host != "laptop-a" || hosts[0].Events != 3 || hosts[0].Verdicts[model.VerdictDeny] != 1 ||
		hosts[1].Host != "laptop-b" || hosts[1].Events != 2 || hosts[1].Verdicts[model.VerdictAsk] != 1 {
		t.Fatalf("hosts = %s", body)
	}
	var events []Stored
	all := get(t, srv.URL+"/v1/events", "tok", &events)
	if len(events) != 5 {
		t.Fatalf("got %d events, want 5", len(events))
	}
	var denied []Stored
	get(t, srv.URL+"/v1/events?verdict=deny&host=laptop-b", "tok", &denied)
	if len(denied) != 1 || denied[0].Host != "laptop-b" {
		t.Fatalf("filtered = %+v", denied)
	}

	// Shipping again sends nothing new.
	before := posts.Load()
	for _, dir := range []string{a, b} {
		if err := sink.Ship(ctx, dir); err != nil {
			t.Fatal(err)
		}
	}
	if posts.Load() != before {
		t.Fatalf("second ship made %d requests, want 0", posts.Load()-before)
	}

	// No raw secret in responses or on disk.
	stored, _ := os.ReadFile(filepath.Join(colDir, FileName))
	for _, s := range testutil.FakeSecrets {
		if strings.Contains(all, s) || strings.Contains(body, s) || bytes.Contains(stored, []byte(s)) {
			t.Fatalf("secret %q leaked", s)
		}
	}
	if st, _ := os.Stat(filepath.Join(colDir, FileName)); st.Mode().Perm() != 0o600 {
		t.Fatalf("events.jsonl mode = %v", st.Mode().Perm())
	}
	t.Logf("two agents reported: laptop-a=3 events, laptop-b=2 events, 0 duplicates, 0 secrets")
}

func TestRetryAfter500DoesNotDuplicate(t *testing.T) {
	store, srv := newCollector(t, t.TempDir(), "")
	var fail atomic.Bool
	fail.Store(true)
	// The collector stores the batch but the reply is lost: the worst case.
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.Config.Handler.ServeHTTP(httptest.NewRecorder(), r)
		if fail.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer flaky.Close()

	dir := agent(t, flaky.URL, "", "laptop-a", model.VerdictAllow, model.VerdictDeny)
	if err := sink.Ship(context.Background(), dir); err == nil {
		t.Fatal("want an error on 500")
	}
	if n, _ := sink.Pending(dir); n != 2 {
		t.Fatalf("pending = %d, want 2", n)
	}
	fail.Store(false)
	if err := sink.Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if got := len(store.Query("", audit.Filter{})); got != 2 {
		t.Fatalf("collector holds %d events, want 2 (no duplicates)", got)
	}
}

func TestRestartKeepsDedup(t *testing.T) {
	colDir := t.TempDir()
	store, srv := newCollector(t, colDir, "")
	dir := agent(t, srv.URL, "", "laptop-a", model.VerdictAllow, model.VerdictDeny)
	if err := sink.Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	store.Close()

	// New collector on the same directory; the agent re-sends everything.
	store2, srv2 := newCollector(t, colDir, "")
	if err := sink.SaveConfig(dir, sink.Config{URL: srv2.URL, Host: "laptop-a"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, sink.OffsetFile)); err != nil {
		t.Fatal(err)
	}
	if err := sink.Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if got := len(store2.Query("", audit.Filter{})); got != 2 {
		t.Fatalf("after restart %d events, want 2", got)
	}
}

func TestAuthAndLimits(t *testing.T) {
	_, srv := newCollector(t, t.TempDir(), "tok")
	post := func(token string, body io.Reader) int {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/v1/events", body)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	ok := `{"host":"h","events":[{"id":"x1","time":"2026-01-01T00:00:00Z","verdict":"allow"}]}`
	big := `{"host":"h","events":[{"id":"x","summary":"` + strings.Repeat("a", MaxBody) + `"}]}`
	tests := []struct {
		name  string
		token string
		body  string
		want  int
	}{
		{"no token", "", ok, http.StatusUnauthorized},
		{"wrong token", "nope", ok, http.StatusUnauthorized},
		{"good", "tok", ok, http.StatusOK},
		{"bad json", "tok", `{`, http.StatusBadRequest},
		{"no host", "tok", `{"events":[]}`, http.StatusBadRequest},
		{"bad verdict", "tok", `{"host":"h","events":[{"id":"y","verdict":"maybe"}]}`, http.StatusBadRequest},
		{"too large", "tok", big, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		if got := post(tt.token, strings.NewReader(tt.body)); got != tt.want {
			t.Errorf("%s: status %d, want %d", tt.name, got, tt.want)
		}
	}
	if code := getStatus(t, srv.URL+"/healthz"); code != http.StatusOK {
		t.Errorf("healthz = %d without token, want 200", code)
	}
	if code := getStatus(t, srv.URL+"/v1/hosts"); code != http.StatusUnauthorized {
		t.Errorf("hosts without token = %d, want 401", code)
	}
}

func getStatus(t *testing.T, url string) int {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestServeShutsDownOnCancel(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ln, err := newLoopbackListener()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, Handler(store, "")) }()
	if code := getStatus(t, "http://"+ln.Addr().String()+"/healthz"); code != http.StatusOK {
		t.Fatalf("healthz = %d", code)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}
