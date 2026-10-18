package sink

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/redact"
)

const (
	// BatchSize is the most events sent in one request.
	BatchSize = 500
	// requestTimeout bounds one HTTP request, so a dead collector can
	// never stall the caller for long.
	requestTimeout = 2 * time.Second
)

// Ship sends the audit decisions that were not shipped yet to the
// configured collector. It is a no-op (and makes no request) when
// <dataDir>/sink.json does not exist, and also when another mcpgraph
// process is shipping right now. Proxies and hooks call it after
// recording a decision. Errors mean "try again later"; callers log them
// and carry on, because shipping must never affect the agent.
func Ship(ctx context.Context, dataDir string) error {
	cfg, on, err := LoadConfig(dataDir)
	if err != nil || !on {
		return err
	}
	unlock, locked, err := lock(dataDir)
	if err != nil || !locked {
		return err
	}
	defer unlock()

	f, err := os.Open(filepath.Join(dataDir, audit.FileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	off := readOffset(dataDir, f)
	client := &http.Client{Timeout: requestTimeout}
	for {
		lines, end, err := nextLines(f, off, BatchSize)
		if err != nil || len(lines) == 0 {
			return err
		}
		if err := post(ctx, client, cfg, buildEvents(lines)); err != nil {
			return err
		}
		off = end
		if err := writeFile(filepath.Join(dataDir, OffsetFile), []byte(strconv.FormatInt(off, 10))); err != nil {
			return fmt.Errorf("save offset: %w", err)
		}
	}
}

// Pending counts the complete audit lines that were not shipped yet.
func Pending(dataDir string) (int, error) {
	f, err := os.Open(filepath.Join(dataDir, audit.FileName))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()
	lines, _, err := nextLines(f, readOffset(dataDir, f), -1)
	return len(lines), err
}

// lock takes an exclusive, non-blocking flock on sink.lock. locked is
// false when another process holds it. flock locks belong to the open
// file, so closing it (unlock) releases the lock, and the kernel releases
// it if the process crashes.
func lock(dir string) (unlock func(), locked bool, err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(filepath.Join(dir, LockFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() { f.Close() }, true, nil
}

// readOffset returns the saved byte offset. A missing, unreadable or
// out-of-range offset (the log was replaced) restarts at 0: re-sending is
// safe because the collector drops duplicates.
func readOffset(dir string, logFile *os.File) int64 {
	b, err := os.ReadFile(filepath.Join(dir, OffsetFile))
	if err != nil {
		return 0
	}
	off, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if st, serr := logFile.Stat(); err != nil || off < 0 || serr != nil || off > st.Size() {
		return 0
	}
	return off
}

// nextLines reads up to max complete lines (all if max < 0) starting at
// off. end is the offset after the last complete line. A last line
// without "\n" may still be mid-write by another process, so it is left
// for the next call.
func nextLines(f *os.File, off int64, max int) (lines [][]byte, end int64, err error) {
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, off, err
	}
	end = off
	r := bufio.NewReader(f)
	for max < 0 || len(lines) < max {
		line, err := r.ReadBytes('\n')
		if err != nil { // io.EOF with a partial line: not complete, drop it
			if errors.Is(err, io.EOF) {
				return lines, end, nil
			}
			return lines, end, err
		}
		end += int64(len(line))
		if line = bytes.TrimSpace(line); len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines, end, nil
}

// buildEvents turns raw audit lines into wire events. The ID hashes the
// raw line, so the same line always gets the same ID. Malformed lines are
// skipped (they are never retried). Summary and reason are redacted again
// before anything leaves the machine.
func buildEvents(lines [][]byte) []Event {
	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		var d model.Decision
		if json.Unmarshal(line, &d) != nil {
			continue
		}
		d.Summary = redact.Text(d.Summary)
		d.Reason = redact.Text(d.Reason)
		sum := sha256.Sum256(line)
		events = append(events, Event{ID: hex.EncodeToString(sum[:]), Decision: d})
	}
	return events
}

// post sends one batch. Only a 2xx counts as delivered.
func post(ctx context.Context, c *http.Client, cfg Config, events []Event) error {
	if len(events) == 0 {
		return nil // only malformed lines: nothing to send, just move on
	}
	body, err := json.Marshal(Batch{Host: cfg.Host, Events: events})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.URL, "/")+"/v1/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("ship events: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the connection can be reused
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("ship events: collector answered %s", resp.Status)
	}
	return nil
}

// Flusher ships in the background for a long-running process (the proxy).
// At most one flush runs at a time; extra requests are dropped because the
// running flush, or the final one in Close, picks up what they would send.
type Flusher struct {
	dir     string
	running atomic.Bool
	wg      sync.WaitGroup
}

// NewFlusher returns a Flusher for dataDir, or nil when the sink is off.
// A nil *Flusher is safe to use: Wrap returns the recorder unchanged.
func NewFlusher(dataDir string) *Flusher {
	if _, on, err := LoadConfig(dataDir); err != nil || !on {
		return nil
	}
	return &Flusher{dir: dataDir}
}

// notify starts a background flush unless one is already running.
func (f *Flusher) notify() {
	if !f.running.CompareAndSwap(false, true) {
		return
	}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		defer f.running.Store(false)
		f.flush(3 * time.Second)
	}()
}

func (f *Flusher) flush(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := Ship(ctx, f.dir); err != nil {
		slog.Warn("mcpgraph sink: not shipped, will retry", "err", err)
	}
}

// Close waits for a running flush, then flushes once more with a short
// deadline so the last decisions leave before the process exits.
func (f *Flusher) Close() {
	if f == nil {
		return
	}
	f.wg.Wait()
	f.flush(2 * time.Second)
}

// Recorder is what the proxy and hook write decisions to.
type Recorder interface {
	Append(model.Decision) error
}

// Wrap returns a Recorder that appends to r and then asks for a flush.
func (f *Flusher) Wrap(r Recorder) Recorder {
	if f == nil {
		return r
	}
	return flushingRecorder{r, f}
}

type flushingRecorder struct {
	Recorder
	f *Flusher
}

func (fr flushingRecorder) Append(d model.Decision) error {
	err := fr.Recorder.Append(d)
	if err == nil {
		fr.f.notify()
	}
	return err
}
