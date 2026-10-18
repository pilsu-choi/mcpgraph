package install

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
)

// Restore modes.
const (
	// ModeRestore: the file is exactly as install left it, so the backup
	// is written back byte for byte.
	ModeRestore = "restore"
	// ModeUnwrap: the file changed after install (apps rewrite their
	// configs, and the user may have edited it), so restoring the backup
	// would throw those changes away. Instead the proxy wrapping is
	// removed in place and everything else is kept.
	ModeUnwrap = "unwrap"
	// ModeGone: the config file no longer exists; nothing to do.
	ModeGone = "gone"
)

// Step undoes one manifest record.
type Step struct {
	Record        Record
	Mode          string
	Before, After []byte
	Edits         []Edit
}

// PlanUninstall works out how to undo every record, newest first, without
// writing anything. Several records for one file are chained: each step
// starts from the result of the previous one.
func PlanUninstall(dataDir string) ([]Step, error) {
	m, err := LoadManifest(dataDir)
	if err != nil {
		return nil, err
	}
	current := map[string][]byte{} // simulated file contents, by real path
	var steps []Step
	for i := len(m.Records) - 1; i >= 0; i-- {
		rec := m.Records[i]
		cur, ok := current[rec.Real]
		if !ok {
			cur, err = os.ReadFile(rec.Real)
			if errors.Is(err, fs.ErrNotExist) {
				steps = append(steps, Step{Record: rec, Mode: ModeGone})
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		step, err := planStep(rec, cur)
		if err != nil {
			return nil, err
		}
		current[rec.Real] = step.After
		steps = append(steps, step)
	}
	return steps, nil
}

func planStep(rec Record, cur []byte) (Step, error) {
	step := Step{Record: rec, Before: cur}
	if sha(cur) == rec.After {
		backup, err := os.ReadFile(rec.Backup)
		if err != nil {
			return step, fmt.Errorf("read backup of %s: %w", rec.Real, err)
		}
		if sha(backup) != rec.Before {
			return step, fmt.Errorf("backup %s does not match the manifest; restore %s by hand", rec.Backup, rec.Real)
		}
		step.Mode = ModeRestore
		step.After = backup
		step.Edits = reverseEdits(backup, rec.Edits)
		// The reverse edits only drive the diff; the file is restored
		// from the backup bytes. If they ever disagree, show the whole
		// file as changed rather than a wrong diff.
		if !bytes.Equal(applyEdits(cur, step.Edits), backup) {
			step.Edits = []Edit{{Start: 0, End: len(cur), New: backup}}
		}
		return step, nil
	}

	step.Mode = ModeUnwrap
	root, err := parseJSON(cur)
	if err != nil {
		return step, fmt.Errorf("%s changed since install and no longer parses: %w", rec.Real, err)
	}
	for _, e := range findEntries(rec.App, root, cur) {
		if !slices.Contains(rec.Servers, e.where) || e.args == nil || e.args.kind != 'a' {
			continue
		}
		cmd, args, ok := config.UnwrapProxyCommand(e.cmd, e.argv)
		if !ok {
			continue // already unwrapped, or replaced by the user
		}
		step.Edits = append(step.Edits, rewrite(e, cmd, args)...)
	}
	step.After = applyEdits(cur, step.Edits)
	return step, nil
}

// ApplyUninstall writes the planned steps and removes their records and
// backups. Files are written once, with the result of their last step.
func ApplyUninstall(dataDir string, steps []Step) error {
	m, err := LoadManifest(dataDir)
	if err != nil {
		return err
	}
	type target struct {
		before, after []byte
		mode          fs.FileMode
	}
	targets := map[string]*target{}
	var order []string
	for _, s := range steps {
		if s.Mode == ModeGone {
			continue
		}
		t, ok := targets[s.Record.Real]
		if !ok {
			t = &target{before: s.Before, mode: fs.FileMode(s.Record.Mode)}
			targets[s.Record.Real] = t
			order = append(order, s.Record.Real)
		}
		t.after = s.After
	}
	for _, path := range order {
		t := targets[path]
		if info, err := os.Stat(path); err == nil {
			t.mode = info.Mode().Perm() // keep the mode the file has now
		}
		if !bytes.Equal(t.before, t.after) {
			if err := writeAtomic(path, t.after, t.mode, sha(t.before)); err != nil {
				return err
			}
		}
		// This file is done: drop its records and backups.
		var keep []Record
		for _, r := range m.Records {
			if r.Real == path {
				removeBackup(r.Backup)
				continue
			}
			keep = append(keep, r)
		}
		m.Records = keep
		if err := m.save(dataDir); err != nil {
			return err
		}
	}
	// Records whose file is gone are dropped too.
	var keep []Record
	for _, r := range m.Records {
		if _, err := os.Stat(r.Real); errors.Is(err, fs.ErrNotExist) {
			removeBackup(r.Backup)
			continue
		}
		keep = append(keep, r)
	}
	m.Records = keep
	return m.save(dataDir)
}

// removeBackup deletes a backup that is no longer needed, and its
// directory once it is empty (os.Remove refuses a non-empty directory).
func removeBackup(path string) {
	_ = os.Remove(path)
	_ = os.Remove(filepath.Dir(path))
}

// Diff prints the diff of one planned install change, with secrets
// redacted.
func (fc FileChange) Diff(w io.Writer) error {
	return writeDiff(w, fc.Real+" (now)", fc.Real+" (after install)", fc.Before, fc.After, fc.Edits)
}

// Diff prints the diff of an uninstall step, with secrets redacted.
func (s Step) Diff(w io.Writer) error {
	return writeDiff(w, s.Record.Real+" (now)", s.Record.Real+" (after uninstall)", s.Before, s.After, s.Edits)
}
