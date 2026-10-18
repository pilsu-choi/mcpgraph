package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/store"
)

type fakeState struct {
	g        *model.Graph
	findings []model.Finding
}

func (f fakeState) Current() (*model.Graph, []model.Finding, bool) {
	return f.g, f.findings, f.g != nil
}

type fakeSnaps struct{ err error }

func (f fakeSnaps) List(_ context.Context, limit int) ([]store.SnapshotMeta, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []store.SnapshotMeta{}
	for i := range min(limit, 3) {
		out = append(out, store.SnapshotMeta{ID: int64(3 - i)})
	}
	return out, nil
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func newTestServer(t *testing.T, st StateSource, snaps SnapshotLister) http.Handler {
	t.Helper()
	s, err := New("127.0.0.1:7878", st, snaps, quiet)
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

// do sends a request with a loopback Host header unless one is given.
func do(h http.Handler, method, target, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	req.Host = "127.0.0.1:7878"
	if host != "" {
		req.Host = host
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestRefusesNonLoopbackAddress is the milestone 8 acceptance test.
func TestRefusesNonLoopbackAddress(t *testing.T) {
	bad := []string{
		"0.0.0.0:7878", // all interfaces
		":7878",        // empty host also means all interfaces
		"[::]:7878",    // all IPv6 interfaces
		"192.168.1.5:7878",
		"localhost:7878", // a name can resolve to anything
		"example.com:80",
		"127.0.0.1",       // no port
		"127.0.0.1:99999", // bad port
	}
	for _, addr := range bad {
		if _, err := New(addr, fakeState{}, fakeSnaps{}, quiet); err == nil {
			t.Errorf("New(%q) succeeded, want refusal", addr)
		}
	}
	if _, err := New("127.0.0.1:0", fakeState{}, fakeSnaps{}, quiet); err != nil {
		t.Errorf("New(127.0.0.1:0) = %v, want ok", err)
	}
}

func TestServeBindsLoopbackOnly(t *testing.T) {
	s, err := New("127.0.0.1:0", fakeState{}, fakeSnaps{}, quiet)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve = %v", err)
	}
}

func TestEndpoints(t *testing.T) {
	g := &model.Graph{Nodes: []model.Node{{ID: "AIApp:1", Kind: model.KindAIApp, Label: "Cursor", App: &model.AIApp{Name: "Cursor"}}}}
	findings := []model.Finding{{RuleID: "R1", Severity: model.SeverityHigh}}
	h := newTestServer(t, fakeState{g: g, findings: findings}, fakeSnaps{})

	rec := do(h, "GET", "/graph", "")
	var gotGraph model.Graph
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &gotGraph) != nil || len(gotGraph.Nodes) != 1 {
		t.Fatalf("/graph: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("/graph headers: %v", rec.Header())
	}

	rec = do(h, "GET", "/findings", "")
	var gotFindings []model.Finding
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &gotFindings) != nil || len(gotFindings) != 1 {
		t.Fatalf("/findings: %d %s", rec.Code, rec.Body)
	}

	rec = do(h, "GET", "/snapshots?limit=2", "")
	var metas []store.SnapshotMeta
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &metas) != nil || len(metas) != 2 {
		t.Fatalf("/snapshots: %d %s", rec.Code, rec.Body)
	}
}

func TestErrorsAndMethods(t *testing.T) {
	ready := newTestServer(t, fakeState{g: &model.Graph{}}, fakeSnaps{})
	notReady := newTestServer(t, fakeState{}, fakeSnaps{err: errors.New("db locked")})

	tests := []struct {
		name   string
		h      http.Handler
		method string
		target string
		host   string
		want   int
	}{
		{"POST is not allowed", ready, "POST", "/graph", "", http.StatusMethodNotAllowed},
		{"DELETE is not allowed", ready, "DELETE", "/snapshots", "", http.StatusMethodNotAllowed},
		{"unknown path", ready, "GET", "/admin", "", http.StatusNotFound},
		{"DNS rebinding host", ready, "GET", "/graph", "evil.example:7878", http.StatusForbidden},
		{"localhost host is fine", ready, "GET", "/graph", "localhost:7878", http.StatusOK},
		{"bad limit", ready, "GET", "/snapshots?limit=0", "", http.StatusBadRequest},
		{"not ready", notReady, "GET", "/graph", "", http.StatusServiceUnavailable},
		{"store error hides details", notReady, "GET", "/snapshots", "", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(tt.h, tt.method, tt.target, tt.host)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.want, rec.Body)
			}
			if strings.Contains(rec.Body.String(), "db locked") {
				t.Error("internal error details leaked to client")
			}
		})
	}
}

// Compile-time check: *store.Store satisfies SnapshotLister.
var _ SnapshotLister = (*store.Store)(nil)
