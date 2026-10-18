// Package audit is the append-only JSONL log of runtime decisions.
//
// Every decision the proxy or the hook makes is one line of JSON. The file
// is only ever appended to: mcpgraph never rewrites or truncates it, so it
// can be shipped, tailed or checksummed by other tools.
package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/redact"
)

// FileName is the audit log's name inside the data directory.
const FileName = "audit.jsonl"

// DefaultDataDir returns mcpgraph's own data directory, the only place it
// writes during normal operation (hard rule 1):
//
//	Linux: $XDG_DATA_HOME/mcpgraph, else ~/.local/share/mcpgraph
//	macOS: ~/Library/Application Support/mcpgraph
func DefaultDataDir(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "mcpgraph")
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "mcpgraph")
	}
	return filepath.Join(home, ".local", "share", "mcpgraph")
}

// Log appends decisions to a JSONL file.
//
// Several mcpgraph processes may append at the same time: every hook call
// is its own process, and each proxied server has its own proxy. The file
// is opened with O_APPEND and each decision is written with a single
// Write call, so the kernel places every line at the current end of file
// and lines from different processes do not overwrite each other.
type Log struct {
	mu sync.Mutex // serialises Append calls within one process
	f  *os.File
}

// Open opens (or creates) the log at path for appending. The directory is
// created with mode 0700 and the file with 0600: decisions describe what
// the user's agents did, which is private even after redaction.
func Open(path string) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create audit directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	return &Log{f: f}, nil
}

// Append writes one decision as one line.
//
// The summary and reason are redacted again here. Callers already pass
// redacted text (policy.Summary), but the audit log is the place a leak
// would last longest, so it does not trust them.
func (l *Log) Append(d model.Decision) error {
	d.Summary = redact.Text(d.Summary)
	d.Reason = redact.Text(d.Reason)
	if d.Time.IsZero() {
		d.Time = time.Now()
	}
	d.Time = d.Time.UTC()

	line, err := jsonLine(d)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.f.Write(line); err != nil {
		return fmt.Errorf("append to audit log: %w", err)
	}
	return nil
}

// jsonLine encodes d as one line of JSON, newline included. Messages
// cannot contain raw newlines: encoding/json escapes them inside strings.
func jsonLine(d model.Decision) ([]byte, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("encode decision: %w", err)
	}
	return append(b, '\n'), nil
}

// Close closes the file.
func (l *Log) Close() error {
	return l.f.Close()
}

// maxLine is the longest line Read accepts. bufio.Scanner's default limit
// is 64 KiB and it fails on longer lines, so raise it explicitly; a
// summary is capped at 200 characters, so real lines are far shorter.
const maxLine = 1 << 20 // 1 MiB

// ReadResult is what Read found.
type ReadResult struct {
	Decisions []model.Decision
	// Skipped counts lines that were not valid JSON, e.g. a last line cut
	// short by a crash. They are skipped instead of failing the read.
	Skipped int
}

// Read parses a JSONL stream and returns the decisions that pass the
// filter, oldest first. With f.Limit > 0 only the newest Limit are kept.
func Read(r io.Reader, f Filter) (ReadResult, error) {
	var res ReadResult
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var d model.Decision
		if err := json.Unmarshal(line, &d); err != nil {
			res.Skipped++
			continue
		}
		if f.Match(d) {
			res.Decisions = append(res.Decisions, d)
		}
	}
	if err := sc.Err(); err != nil {
		return res, fmt.Errorf("read audit log: %w", err)
	}
	if f.Limit > 0 && len(res.Decisions) > f.Limit {
		res.Decisions = res.Decisions[len(res.Decisions)-f.Limit:]
	}
	return res, nil
}

// ReadFile reads the log at path. A missing file is an empty log, not an
// error: nothing has been decided yet.
func ReadFile(path string, f Filter) (ReadResult, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return ReadResult{}, nil
	}
	if err != nil {
		return ReadResult{}, fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()
	return Read(file, f)
}
