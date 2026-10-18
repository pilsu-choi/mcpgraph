package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pilsuchoi/mcpgraph/internal/collector"
	"github.com/pilsuchoi/mcpgraph/internal/collector/config"
	"github.com/pilsuchoi/mcpgraph/internal/graph"
	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/report"
	"github.com/pilsuchoi/mcpgraph/internal/rules"
	"github.com/pilsuchoi/mcpgraph/internal/store"
	"github.com/pilsuchoi/mcpgraph/internal/testutil"
)

// scanFixtures runs a full scan over a fake home filled with every fixture.
func scanFixtures(t *testing.T) *model.Graph {
	t.Helper()
	home := testutil.FakeHome(t)
	g, err := Run(context.Background(), []collector.Collector{
		collector.Host{},
		config.New(home, "linux"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// renderAll renders the graph in every output format mcpgraph produces.
// When a new format is added, add it here so the leak test covers it.
func renderAll(t *testing.T, g *model.Graph) map[string]string {
	t.Helper()
	out := map[string]string{}

	var inv bytes.Buffer
	if err := report.Inventory(&inv, g); err != nil {
		t.Fatal(err)
	}
	out["inventory"] = inv.String()

	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	out["graph-json"] = string(raw)

	var exp bytes.Buffer
	if err := report.JSON(&exp, g); err != nil {
		t.Fatal(err)
	}
	out["export-json"] = exp.String()

	var mer bytes.Buffer
	if err := report.Mermaid(&mer, g); err != nil {
		t.Fatal(err)
	}
	out["mermaid"] = mer.String()

	findings := rules.Evaluate(g, rules.Default("/home/u"))
	var fb bytes.Buffer
	if err := report.Findings(&fb, findings); err != nil {
		t.Fatal(err)
	}
	out["findings-table"] = fb.String()
	fj, err := json.Marshal(findings)
	if err != nil {
		t.Fatal(err)
	}
	out["findings-json"] = string(fj)

	// The SQLite file on disk, read as raw bytes.
	dbPath := filepath.Join(t.TempDir(), "leak.db")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Save(context.Background(), time.Now(), g, findings, graph.Diff(nil, g)); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	var db strings.Builder
	// WAL mode keeps recent writes in the -wal file until checkpoint.
	for _, p := range []string{dbPath, dbPath + "-wal"} {
		if raw, err := os.ReadFile(p); err == nil {
			db.Write(raw)
		}
	}
	out["sqlite"] = db.String()
	return out
}

// TestNoSecretInAnyOutput is the acceptance test for hard rule 2: fake
// secrets planted in the fixtures must never appear in any output.
func TestNoSecretInAnyOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use Linux paths")
	}
	g := scanFixtures(t)

	creds := g.NodesOfKind(model.KindCredential)
	if len(creds) < len(testutil.FakeSecrets) {
		t.Fatalf("found %d credentials, expected at least %d: detection is not working", len(creds), len(testutil.FakeSecrets))
	}

	for format, text := range renderAll(t, g) {
		for _, secret := range testutil.FakeSecrets {
			if strings.Contains(text, secret) {
				t.Errorf("%s output contains raw secret %q", format, secret)
			}
			// Also check that no 8-character slice of the secret beyond
			// the allowed last 4 leaks (e.g. a half-masked value).
			for i := 0; i+8 <= len(secret)-4; i++ {
				if strings.Contains(text, secret[i:i+8]) && !isCommonWord(secret[i:i+8]) {
					t.Errorf("%s output contains fragment %q of secret %q", format, secret[i:i+8], secret)
					break
				}
			}
		}
	}
}

// isCommonWord skips fragments that are legitimately present elsewhere in
// the output (e.g. "FAKEFAKE" could appear in a masked tail).
func isCommonWord(s string) bool {
	return strings.Trim(s, "FAKE") == ""
}
