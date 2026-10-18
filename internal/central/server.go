package central

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/redact"
	"github.com/pilsu-choi/mcpgraph/internal/sink"
)

// Limits on what one request may carry.
const (
	MaxBody      = 4 << 20 // bytes
	maxEvents    = 1000
	maxHostBytes = 128
)

// Handler returns the collector's HTTP API over store. An empty token
// disables authentication (loopback use); otherwise every /v1 request
// needs "Authorization: Bearer <token>". /healthz is always open.
func Handler(store *Store, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/events", auth(token, func(w http.ResponseWriter, r *http.Request) { postEvents(store, w, r) }))
	mux.HandleFunc("GET /v1/events", auth(token, func(w http.ResponseWriter, r *http.Request) { getEvents(store, w, r) }))
	mux.HandleFunc("GET /v1/hosts", auth(token, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, store.Hosts())
	}))
	return mux
}

// auth wraps h with the bearer-token check. ConstantTimeCompare takes the
// same time wherever the strings differ, so the token cannot be guessed
// byte by byte from response times.
func auth(token string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				writeError(w, http.StatusUnauthorized, "missing or wrong bearer token")
				return
			}
		}
		h(w, r)
	}
}

func postEvents(store *Store, w http.ResponseWriter, r *http.Request) {
	// MaxBytesReader makes Decode fail once the body passes the limit.
	r.Body = http.MaxBytesReader(w, r.Body, MaxBody)
	var b sink.Batch
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		var tooBig *http.MaxBytesError // errors.As finds the typed error inside wrapped ones
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "body larger than 4 MiB")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg := validate(&b); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	accepted, err := store.Add(b.Host, b.Events, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot store events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"accepted": accepted, "duplicates": len(b.Events) - accepted})
}

// validate checks a batch and re-redacts summary and reason (the sink
// already did; a different or older client may not have). It returns a
// message for the first problem, or "".
func validate(b *sink.Batch) string {
	switch {
	case b.Host == "" || len(b.Host) > maxHostBytes:
		return "host is required (at most 128 bytes)"
	case len(b.Events) > maxEvents:
		return "too many events in one batch (max 1000)"
	}
	for i := range b.Events {
		e := &b.Events[i]
		switch e.Verdict {
		case model.VerdictAllow, model.VerdictDeny, model.VerdictAsk:
		default:
			return "event " + strconv.Itoa(i) + ": unknown verdict"
		}
		if e.ID == "" || len(e.ID) > 128 {
			return "event " + strconv.Itoa(i) + ": id is required"
		}
		e.Summary = redact.Text(e.Summary)
		e.Reason = redact.Text(e.Reason)
	}
	return ""
}

func getEvents(store *Store, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := audit.Filter{Verdict: model.Verdict(q.Get("verdict")), Source: q.Get("source")}
	if s := q.Get("since"); s != "" {
		t, err := parseSince(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "since: want a duration (24h) or an RFC 3339 time")
			return
		}
		f.Since = t
	}
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "limit: want a non-negative number")
			return
		}
		f.Limit = n
	}
	writeJSON(w, http.StatusOK, store.Query(q.Get("host"), f))
}

func parseSince(s string) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	return time.Parse(time.RFC3339, s)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) // the client may be gone; nothing useful to do
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// Serve serves h on ln until ctx is cancelled, then shuts down gracefully
// (in-flight requests get 5 s to finish). The timeouts stop slow or stuck
// clients from holding connections forever.
func Serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// ctx is already cancelled, so the shutdown deadline needs a fresh one.
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Query fetches events from a collector (used by `mcpgraph events
// --collector`). params may hold host, verdict, source, since and limit.
func Query(ctx context.Context, baseURL, token string, params url.Values) ([]Stored, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/events?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("collector answered " + resp.Status)
	}
	var out []Stored
	return out, json.NewDecoder(resp.Body).Decode(&out)
}
