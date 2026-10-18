package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/collector"
	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
	"github.com/pilsu-choi/mcpgraph/internal/graph"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/posture"
	"github.com/pilsu-choi/mcpgraph/internal/report"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// scanFixtures runs a full scan over a fake home filled with every
// fixture, plus an audit log of decisions replayed from secretActions.
func scanFixtures(t *testing.T) (*model.Graph, string) {
	t.Helper()
	home := testutil.FakeHome(t)
	auditPath := replayDecisions(t, secretActions())
	g, err := Run(context.Background(), []collector.Collector{
		collector.Host{},
		config.New(home, "linux"),
		audit.Collector{Path: auditPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	return g, auditPath
}

// renderAll renders the graph in every output format mcpgraph produces.
// When a new format is added, add it here so the leak test covers it.
func renderAll(t *testing.T, g *model.Graph, auditPath string) map[string]string {
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

	findings := posture.Evaluate(g, posture.Default("/home/u"))
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

	// The audit log file itself, and both `mcpgraph events` formats.
	raw, err = os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	out["audit-jsonl"] = string(raw)
	res, err := audit.ReadFile(auditPath, audit.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var ev, evj bytes.Buffer
	if err := report.Events(&ev, res.Decisions); err != nil {
		t.Fatal(err)
	}
	if err := report.EventsJSONL(&evj, res.Decisions); err != nil {
		t.Fatal(err)
	}
	out["events-table"] = ev.String()
	out["events-jsonl"] = evj.String()

	// Diff messages are what drift alerts (milestone 12) will print.
	diff, err := json.Marshal(graph.Diff(nil, g))
	if err != nil {
		t.Fatal(err)
	}
	out["diff-json"] = string(diff)
	return out
}

// TestNoSecretInAnyOutput is the acceptance test for hard rule 2: fake
// secrets planted in the fixtures must never appear in any output.
func TestNoSecretInAnyOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use Linux paths")
	}
	g, auditPath := scanFixtures(t)

	creds := g.NodesOfKind(model.KindCredential)
	if len(creds) < len(testutil.FakeSecrets) {
		t.Fatalf("found %d credentials, expected at least %d: detection is not working", len(creds), len(testutil.FakeSecrets))
	}

	for format, text := range renderAll(t, g, auditPath) {
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
