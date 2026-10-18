// Package store keeps graph snapshots in a local SQLite database.
//
// It uses modernc.org/sqlite, a pure-Go SQLite, so the binary still builds
// with CGO_ENABLED=0. The database lives in mcpgraph's own data directory,
// the only place mcpgraph ever writes (AGENTS.md hard rule 1).
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// The blank import runs the package's init(), which registers the
	// "sqlite" driver with database/sql. We never call it directly.
	_ "modernc.org/sqlite"

	"github.com/pilsuchoi/mcpgraph/internal/graph"
	"github.com/pilsuchoi/mcpgraph/internal/model"
)

// DefaultKeep is how many snapshots are kept before old ones are pruned.
const DefaultKeep = 500

const schema = `
CREATE TABLE IF NOT EXISTS snapshots (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	taken_at   TEXT    NOT NULL,
	node_count INTEGER NOT NULL,
	high       INTEGER NOT NULL,
	medium     INTEGER NOT NULL,
	graph      TEXT    NOT NULL,
	findings   TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS changes (
	snapshot_id INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
	type        TEXT    NOT NULL,
	kind        TEXT    NOT NULL,
	node_id     TEXT    NOT NULL,
	message     TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS changes_snapshot ON changes(snapshot_id);
`

// Store is a snapshot database. Its methods are safe for concurrent use
// because *sql.DB is a connection pool that handles locking itself.
type Store struct {
	db   *sql.DB
	keep int
}

// SnapshotMeta summarises one snapshot without loading the graph.
type SnapshotMeta struct {
	ID        int64          `json:"id"`
	TakenAt   time.Time      `json:"taken_at"`
	NodeCount int            `json:"node_count"`
	High      int            `json:"high"`
	Medium    int            `json:"medium"`
	Changes   []graph.Change `json:"changes,omitempty"`
}

// Open opens (or creates) the database at path. The parent directory is
// created with owner-only permissions.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	// busy_timeout makes concurrent writers wait instead of failing;
	// foreign_keys enables ON DELETE CASCADE.
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Store{db: db, keep: DefaultKeep}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Save stores a snapshot with its findings and the changes since the
// previous snapshot, then prunes old snapshots. It returns the new ID.
//
// The graph is already redacted, so nothing secret reaches the disk.
func (s *Store) Save(ctx context.Context, takenAt time.Time, g *model.Graph, findings []model.Finding, changes []graph.Change) (int64, error) {
	graphJSON, err := json.Marshal(g)
	if err != nil {
		return 0, err
	}
	findingsJSON, err := json.Marshal(findings)
	if err != nil {
		return 0, err
	}
	high, medium := 0, 0
	for _, f := range findings {
		switch f.Severity {
		case model.SeverityHigh:
			high++
		case model.SeverityMedium:
			medium++
		}
	}

	// A transaction makes the snapshot and its changes appear together or
	// not at all. The deferred Rollback is a no-op after Commit succeeds,
	// and cleans up on every early return.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // error after Commit is expected and meaningless

	res, err := tx.ExecContext(ctx,
		`INSERT INTO snapshots (taken_at, node_count, high, medium, graph, findings) VALUES (?, ?, ?, ?, ?, ?)`,
		takenAt.UTC().Format(time.RFC3339Nano), len(g.Nodes), high, medium, string(graphJSON), string(findingsJSON))
	if err != nil {
		return 0, fmt.Errorf("insert snapshot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, c := range changes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO changes (snapshot_id, type, kind, node_id, message) VALUES (?, ?, ?, ?, ?)`,
			id, c.Type, string(c.Kind), c.NodeID, c.Message); err != nil {
			return 0, fmt.Errorf("insert change: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM snapshots WHERE id <= (SELECT MAX(id) FROM snapshots) - ?`, s.keep); err != nil {
		return 0, fmt.Errorf("prune: %w", err)
	}
	return id, tx.Commit()
}

// Latest returns the most recent graph, or nil if there is none yet.
func (s *Store) Latest(ctx context.Context) (*model.Graph, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT graph FROM snapshots ORDER BY id DESC LIMIT 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var g model.Graph
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	return &g, nil
}

// List returns up to limit snapshots, newest first, with their changes.
func (s *Store) List(ctx context.Context, limit int) ([]SnapshotMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, taken_at, node_count, high, medium FROM snapshots ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	// rows must be closed to return the connection to the pool; defer
	// guarantees that even if we return early with an error.
	defer rows.Close()

	out := []SnapshotMeta{}
	for rows.Next() {
		var m SnapshotMeta
		var takenAt string
		if err := rows.Scan(&m.ID, &takenAt, &m.NodeCount, &m.High, &m.Medium); err != nil {
			return nil, err
		}
		m.TakenAt, _ = time.Parse(time.RFC3339Nano, takenAt)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Load changes after closing the first query: SQLite allows only one
	// writer, and keeping two result sets open is easy to get wrong.
	rows.Close()
	for i := range out {
		changes, err := s.changes(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Changes = changes
	}
	return out, nil
}

func (s *Store) changes(ctx context.Context, id int64) ([]graph.Change, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT type, kind, node_id, message FROM changes WHERE snapshot_id = ? ORDER BY rowid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []graph.Change
	for rows.Next() {
		var c graph.Change
		var kind string
		if err := rows.Scan(&c.Type, &kind, &c.NodeID, &c.Message); err != nil {
			return nil, err
		}
		c.Kind = model.NodeKind(kind)
		out = append(out, c)
	}
	return out, rows.Err()
}
