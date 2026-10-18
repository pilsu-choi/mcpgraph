// Package central is the minimal collector: an HTTP server that stores
// decisions shipped by the sink of several machines.
//
// Events live in one append-only file, events.jsonl. The whole file is
// loaded into memory at startup (this is a small self-hosted collector,
// not a database), which also rebuilds the set of event IDs used to drop
// duplicates after a restart.
package central

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/sink"
)

// FileName is the collector's event file inside its data directory.
const FileName = "events.jsonl"

// Stored is an event as the collector keeps and returns it: the shipped
// event plus the sending host and the time the collector received it.
type Stored struct {
	Host     string    `json:"host"`
	Received time.Time `json:"received"`
	sink.Event
}

// HostSummary is one row of GET /v1/hosts.
type HostSummary struct {
	Host     string                `json:"host"`
	LastSeen time.Time             `json:"last_seen"`
	Events   int                   `json:"events"`
	Verdicts map[model.Verdict]int `json:"verdicts"`
}

// Store is the collector's event storage. All methods are safe for
// concurrent use.
type Store struct {
	mu     sync.Mutex
	f      *os.File
	seen   map[string]struct{}
	events []Stored
}

// OpenStore opens (or creates) DIR/events.jsonl and loads it. The file is
// created 0600 and opened O_APPEND.
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create collector directory: %w", err)
	}
	path := filepath.Join(dir, FileName)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	s := &Store{f: f, seen: map[string]struct{}{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20) // default 64 KiB line limit is too small to be safe
	for sc.Scan() {
		var e Stored
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.ID != "" {
			s.seen[e.ID] = struct{}{}
			s.events = append(s.events, e)
		}
	}
	if err := sc.Err(); err != nil {
		f.Close()
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return s, nil
}

// Close closes the event file.
func (s *Store) Close() error { return s.f.Close() }

// Add stores the events of host that are not stored yet and reports how
// many were new. The new lines are written with one Write call; IDs are
// remembered only after the write succeeded, so a failed write can be
// retried by the sender.
func (s *Store) Add(host string, events []sink.Event, now time.Time) (accepted int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		buf   []byte
		fresh []Stored
		batch = map[string]struct{}{} // duplicates inside this request
	)
	for _, ev := range events {
		_, dup := s.seen[ev.ID]
		if _, dupInBatch := batch[ev.ID]; dup || dupInBatch {
			continue
		}
		batch[ev.ID] = struct{}{}
		st := Stored{Host: host, Received: now.UTC(), Event: ev}
		line, err := json.Marshal(st)
		if err != nil {
			return 0, err
		}
		buf = append(append(buf, line...), '\n')
		fresh = append(fresh, st)
	}
	if len(fresh) == 0 {
		return 0, nil
	}
	if _, err := s.f.Write(buf); err != nil {
		return 0, fmt.Errorf("append event: %w", err)
	}
	for _, st := range fresh {
		s.seen[st.ID] = struct{}{}
	}
	s.events = append(s.events, fresh...)
	return len(fresh), nil
}

// Query returns the stored events of host ("" = all hosts) that pass the
// filter, oldest first; f.Limit keeps the newest N. It reuses
// audit.Filter, so the collector and `mcpgraph events` filter alike.
func (s *Store) Query(host string, f audit.Filter) []Stored {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Stored{}
	for _, e := range s.events {
		if (host == "" || e.Host == host) && f.Match(e.Decision) {
			out = append(out, e)
		}
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[len(out)-f.Limit:]
	}
	return out
}

// Hosts summarises every host that reported, sorted by name.
func (s *Store) Hosts() []HostSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	byHost := map[string]*HostSummary{}
	for _, e := range s.events {
		h := byHost[e.Host]
		if h == nil {
			h = &HostSummary{Host: e.Host, Verdicts: map[model.Verdict]int{}}
			byHost[e.Host] = h
		}
		h.Events++
		h.Verdicts[e.Verdict]++
		if e.Received.After(h.LastSeen) {
			h.LastSeen = e.Received
		}
	}
	out := make([]HostSummary, 0, len(byHost))
	for _, h := range byHost {
		out = append(out, *h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}
