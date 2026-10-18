package sink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// writeLog appends n decisions to dir's audit log.
func writeLog(t *testing.T, dir string, n int) {
	t.Helper()
	l, err := audit.Open(filepath.Join(dir, audit.FileName))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for i := 0; i < n; i++ {
		d := model.Decision{Source: "proxy", Agent: "pg", Tool: "query", Summary: "select " + string(rune('a'+i)), Verdict: model.VerdictAllow}
		if err := l.Append(d); err != nil {
			t.Fatal(err)
		}
	}
}

func setSink(t *testing.T, dir, url, token string) {
	t.Helper()
	if err := SaveConfig(dir, Config{URL: url, Token: token, Host: "h1"}); err != nil {
		t.Fatal(err)
	}
}

func offset(t *testing.T, dir string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(dir, OffsetFile))
	return string(b)
}

func TestShipOffMakesNoRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("sink is off, but the server was hit")
	}))
	defer srv.Close()
	dir := t.TempDir()
	writeLog(t, dir, 3)
	if err := Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	// Turned on and off again: still nothing.
	setSink(t, dir, srv.URL, "")
	if err := RemoveConfig(dir); err != nil {
		t.Fatal(err)
	}
	if err := Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
}

func TestShipBadTokenKeepsOffset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			http.Error(w, "no", http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	writeLog(t, dir, 2)

	setSink(t, dir, srv.URL, "bad")
	if err := Ship(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want 401", err)
	}
	if got := offset(t, dir); got != "" {
		t.Fatalf("offset advanced to %q after a 401", got)
	}
	if n, _ := Pending(dir); n != 2 {
		t.Fatalf("pending = %d, want 2", n)
	}

	setSink(t, dir, srv.URL, "good")
	if err := Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if n, _ := Pending(dir); n != 0 {
		t.Fatalf("pending = %d after shipping, want 0", n)
	}
}

func TestShipSkipsPartialLastLine(t *testing.T) {
	var got atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b Batch
		_ = json.NewDecoder(r.Body).Decode(&b)
		got.Add(int32(len(b.Events)))
	}))
	defer srv.Close()
	dir := t.TempDir()
	writeLog(t, dir, 1)
	f, _ := os.OpenFile(filepath.Join(dir, audit.FileName), os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString(`{"time":"2026-01-01T00:00:00Z","verdict":"al`) // writer still busy
	f.Close()

	setSink(t, dir, srv.URL, "")
	if err := Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if got.Load() != 1 {
		t.Fatalf("shipped %d events, want 1 (partial line must wait)", got.Load())
	}
	if n, _ := Pending(dir); n != 0 {
		t.Fatalf("pending = %d, partial line is not complete so not pending", n)
	}
}

func TestShipConcurrentCallsDoNotDoubleSend(t *testing.T) {
	var got atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b Batch
		_ = json.NewDecoder(r.Body).Decode(&b)
		got.Add(int32(len(b.Events)))
		time.Sleep(50 * time.Millisecond) // keep the lock held while the others try
	}))
	defer srv.Close()
	dir := t.TempDir()
	writeLog(t, dir, 5)
	setSink(t, dir, srv.URL, "")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Ship(context.Background(), dir); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got.Load() != 5 {
		t.Fatalf("server received %d events, want exactly 5", got.Load())
	}
}

func TestShipRedactsAgain(t *testing.T) {
	var body strings.Builder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b Batch
		_ = json.NewDecoder(r.Body).Decode(&b)
		out, _ := json.Marshal(b)
		body.Write(out)
	}))
	defer srv.Close()
	dir := t.TempDir()
	// A line written by something that did not redact.
	raw := `{"time":"2026-01-01T00:00:00Z","source":"hook","agent":"a","tool":"Bash","summary":"export TOKEN=` + testutil.FakeSecrets[0] + `","verdict":"deny"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, audit.FileName), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	setSink(t, dir, srv.URL, "")
	if err := Ship(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if body.Len() == 0 || strings.Contains(body.String(), testutil.FakeSecrets[0]) {
		t.Fatalf("body = %q, want a redacted event", body.String())
	}
}

func TestConfig(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
		warn    bool
	}{
		{"http://127.0.0.1:7600", false, false},
		{"https://collector.example.com", false, false},
		{"http://collector.example.com", false, true},
		{"ftp://x", true, false},
		{"collector", true, false},
	}
	for _, tt := range tests {
		warn, err := CheckURL(tt.url)
		if (err != nil) != tt.wantErr || (warn != "") != tt.warn {
			t.Errorf("CheckURL(%q) = %q, %v", tt.url, warn, err)
		}
	}
	dir := t.TempDir()
	if err := SaveConfig(dir, Config{URL: "http://localhost:1", Token: "secret-token-value", Host: "h"}); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(filepath.Join(dir, ConfigFile))
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", st.Mode().Perm())
	}
	c, on, err := LoadConfig(dir)
	if err != nil || !on || c.Host != "h" {
		t.Fatalf("LoadConfig = %+v, %v, %v", c, on, err)
	}
	if m := c.MaskedToken(); strings.Contains(m, "token-value") {
		t.Fatalf("masked token leaks: %s", m)
	}
}

func TestFlusherShipsOnAppendAndClose(t *testing.T) {
	var got atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b Batch
		_ = json.NewDecoder(r.Body).Decode(&b)
		got.Add(int32(len(b.Events)))
	}))
	defer srv.Close()
	dir := t.TempDir()
	if NewFlusher(dir) != nil {
		t.Fatal("sink is off: NewFlusher should return nil")
	}
	setSink(t, dir, srv.URL, "")
	l, err := audit.Open(filepath.Join(dir, audit.FileName))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	fl := NewFlusher(dir)
	rec := fl.Wrap(l)
	for i := 0; i < 3; i++ {
		if err := rec.Append(model.Decision{Source: "proxy", Tool: "q", Summary: string(rune('a' + i)), Verdict: model.VerdictAllow}); err != nil {
			t.Fatal(err)
		}
	}
	fl.Close() // waits for background flushes, then flushes the rest
	if got.Load() != 3 {
		t.Fatalf("shipped %d events, want 3", got.Load())
	}
}
