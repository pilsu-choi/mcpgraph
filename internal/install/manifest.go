package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Dir is the subdirectory of the data directory that install owns.
const Dir = "install"

// Record describes one file that install changed. It holds hashes and
// offsets only, never config contents, so the manifest itself cannot leak
// a secret. The original bytes live in the backup file.
type Record struct {
	Time    time.Time    `json:"time"`
	App     string       `json:"app"`
	Path    string       `json:"path"`
	Real    string       `json:"real"`
	Backup  string       `json:"backup"`
	Mode    uint32       `json:"mode"`
	Before  string       `json:"sha256_before"`
	After   string       `json:"sha256_after"`
	Edits   []EditRecord `json:"edits"`
	Servers []string     `json:"servers"`
}

// EditRecord is an Edit without its new bytes: enough to locate the
// change again in the installed file.
type EditRecord struct {
	Start  int `json:"start"`
	End    int `json:"end"`
	NewLen int `json:"new_len"`
}

// Manifest lists the records, oldest first.
type Manifest struct {
	Records []Record `json:"records"`
}

func manifestPath(dataDir string) string { return filepath.Join(dataDir, Dir, "manifest.json") }

// LoadManifest reads the manifest; a missing one is empty.
func LoadManifest(dataDir string) (*Manifest, error) {
	data, err := os.ReadFile(manifestPath(dataDir))
	if errors.Is(err, fs.ErrNotExist) {
		return &Manifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestPath(dataDir), err)
	}
	return &m, nil
}

func (m *Manifest) save(dataDir string) error {
	if m.Records == nil {
		m.Records = []Record{} // write [], not null
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := manifestPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), 0o600, "")
}

// writeAtomic replaces path with data: write a temporary file in the same
// directory, flush it to disk, then rename it over the target. A rename
// within one directory is atomic, so a reader (or a crash) sees either the
// old file or the new one, never half of each.
//
// When expectSHA is not empty, the current file must still have that hash
// just before the rename; otherwise someone (the AI app itself) changed it
// after we read it, and we stop instead of losing their change.
func writeAtomic(path string, data []byte, mode fs.FileMode, expectSHA string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mcpgraph-*.tmp")
	if err != nil {
		return err
	}
	// If anything below fails, the temporary file is removed. After a
	// successful rename, Remove fails harmlessly.
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	if expectSHA != "" {
		cur, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if sha(cur) != expectSHA {
			return fmt.Errorf("%s changed while mcpgraph was working on it; nothing was written, run the command again", path)
		}
	}
	return os.Rename(tmp.Name(), path)
}
