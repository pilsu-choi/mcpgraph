package install

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Apply backs up and rewrites each planned file, recording each one in the
// manifest as soon as it is written, so a failure halfway leaves a
// manifest that uninstall can still use.
func Apply(dataDir string, changes []FileChange, now time.Time) ([]Record, error) {
	m, err := LoadManifest(dataDir)
	if err != nil {
		return nil, err
	}
	stamp := now.UTC().Format("20060102T150405Z")
	root := filepath.Join(dataDir, Dir, "backups")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	// MkdirTemp adds a random suffix, so two installs in the same second
	// get different directories. It creates the directory with mode 0700.
	backupDir, err := os.MkdirTemp(root, stamp+"-")
	if err != nil {
		return nil, err
	}

	var done []Record
	for i, fc := range changes {
		// The manifest records which file each backup belongs to, so the
		// name only needs to be unique and readable.
		backup := filepath.Join(backupDir, fmt.Sprintf("%d-%s", i+1, filepath.Base(fc.Real)))
		// The backup holds the config exactly as it was, secrets included,
		// so it is readable by the owner only (0600, in a 0700 directory).
		// O_EXCL: never overwrite an existing backup.
		if err := writeNew(backup, fc.Before); err != nil {
			return done, fmt.Errorf("back up %s: %w", fc.Real, err)
		}
		if err := writeAtomic(fc.Real, fc.After, fc.Mode, sha(fc.Before)); err != nil {
			_ = os.Remove(backup) // nothing was changed, so the backup is not needed
			return done, fmt.Errorf("write %s: %w", fc.Real, err)
		}
		rec := Record{
			Time: now.UTC(), App: fc.App, Path: fc.Path, Real: fc.Real, Backup: backup,
			Mode: uint32(fc.Mode), Before: sha(fc.Before), After: sha(fc.After), Servers: fc.Servers,
		}
		for _, e := range fc.Edits {
			rec.Edits = append(rec.Edits, EditRecord{Start: e.Start, End: e.End, NewLen: len(e.New)})
		}
		m.Records = append(m.Records, rec)
		if err := m.save(dataDir); err != nil {
			return done, fmt.Errorf("save manifest (the backup of %s is %s): %w", fc.Real, backup, err)
		}
		done = append(done, rec)
	}
	return done, nil
}

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
