// Package api serves the current graph, findings and snapshot history as
// read-only JSON on 127.0.0.1.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/store"
)

// LoopbackIP is the only address the API may bind (AGENTS.md hard rule 3).
const LoopbackIP = "127.0.0.1"

// StateSource provides the latest scan result. ok is false before the
// first scan has finished.
//
// The api package defines the small interfaces it needs instead of
// importing the daemon: any type with these methods works, which also
// makes the handlers easy to test with fakes.
type StateSource interface {
	Current() (g *model.Graph, findings []model.Finding, ok bool)
}

// SnapshotLister lists stored snapshots.
type SnapshotLister interface {
	List(ctx context.Context, limit int) ([]store.SnapshotMeta, error)
}

// Server is the HTTP API.
type Server struct {
	addr  string
	state StateSource
	snaps SnapshotLister
	log   *slog.Logger
}

// New validates addr and returns a Server. It refuses any address that is
// not 127.0.0.1:port, including "localhost" (which may resolve to another
// interface), "0.0.0.0" and ":port" (which means all interfaces).
func New(addr string, state StateSource, snaps SnapshotLister, log *slog.Logger) (*Server, error) {
	if err := ValidateAddr(addr); err != nil {
		return nil, err
	}
	return &Server{addr: addr, state: state, snaps: snaps, log: log}, nil
}

// ValidateAddr returns an error unless addr is "127.0.0.1:<port>".
func ValidateAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	if host != LoopbackIP {
		return fmt.Errorf("refusing to listen on %q: only %s is allowed", addr, LoopbackIP)
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return fmt.Errorf("invalid port in %q", addr)
	}
	return nil
}

// Handler returns the HTTP routes. Go 1.22+ patterns such as "GET /graph"
// match the method too, so POST/PUT/DELETE get 405 Method Not Allowed
// automatically: the API is read-only by construction.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /graph", s.handleGraph)
	mux.HandleFunc("GET /findings", s.handleFindings)
	mux.HandleFunc("GET /snapshots", s.handleSnapshots)
	return s.checkHost(mux)
}

// Serve listens on the validated address and serves until ctx is
// cancelled, then shuts down gracefully.
func (s *Server) Serve(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}
	// Belt and braces: check the address we actually got.
	if tcp, ok := ln.Addr().(*net.TCPAddr); !ok || !tcp.IP.IsLoopback() {
		ln.Close()
		return fmt.Errorf("listener bound to non-loopback address %s", ln.Addr())
	}

	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	// Shut down in a separate goroutine when ctx is cancelled; Serve
	// below then returns http.ErrServerClosed.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx) //nolint:errcheck // best effort on exit
	}()

	s.log.Info("API listening", "addr", ln.Addr().String())
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// checkHost rejects requests whose Host header is not this loopback
// server. Binding to 127.0.0.1 alone does not stop a malicious web page
// from reaching us through DNS rebinding (evil.com resolving to
// 127.0.0.1); such requests still carry Host: evil.com.
func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != LoopbackIP && host != "localhost" {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	g, _, ok := s.state.Current()
	if !ok {
		http.Error(w, "first scan not finished", http.StatusServiceUnavailable)
		return
	}
	// Snapshot: hand the encoder a private copy of the graph.
	s.writeJSON(w, g.Snapshot())
}

func (s *Server) handleFindings(w http.ResponseWriter, r *http.Request) {
	_, findings, ok := s.state.Current()
	if !ok {
		http.Error(w, "first scan not finished", http.StatusServiceUnavailable)
		return
	}
	if findings == nil {
		findings = []model.Finding{}
	}
	s.writeJSON(w, findings)
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			http.Error(w, "limit must be 1..500", http.StatusBadRequest)
			return
		}
		limit = n
	}
	metas, err := s.snaps.List(r.Context(), limit)
	if err != nil {
		s.log.Error("list snapshots", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, metas)
}

func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		s.log.Warn("write response", "err", err)
	}
}
