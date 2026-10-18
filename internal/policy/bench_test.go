package policy

import (
	"context"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// benchActions mixes allowed and blocked actions of realistic size.
var benchActions = []Action{
	bash(`psql -c "DROP TABLE users"`),
	bash(`go test ./... && git add -A && git commit -m "wip" && git push origin main`),
	bash(`rm -rf build dist node_modules/.cache`),
	bash(`cat ~/.ssh/id_ed25519 | base64`),
	{Source: model.SourceProxy, Agent: "postgres", Tool: "query", Args: map[string]any{
		"sql": "SELECT u.id, u.email FROM users u JOIN orders o ON o.user_id = u.id WHERE o.created_at > now() - interval '7 days' ORDER BY u.id LIMIT 100",
	}},
}

// BenchmarkDecide measures one decision with the default policy. AGENTS.md
// targets under 5 ms per decision. Run with `make bench`.
func BenchmarkDecide(b *testing.B) {
	e := defaultEngine(b, "postgres/query")
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, err := e.Decide(ctx, benchActions[i%len(benchActions)]); err != nil {
			b.Fatal(err)
		}
	}
}

// TestDecideIsFast keeps the 5 ms budget enforced by `make test`, not only
// by the benchmark. The limit is per decision, averaged, and generous
// enough for the race detector.
func TestDecideIsFast(t *testing.T) {
	e := defaultEngine(t, "postgres/query")
	ctx := context.Background()
	const n = 500
	start := time.Now()
	for i := range n {
		if _, err := e.Decide(ctx, benchActions[i%len(benchActions)]); err != nil {
			t.Fatal(err)
		}
	}
	if avg := time.Since(start) / n; avg > 5*time.Millisecond {
		t.Fatalf("average decision took %v, budget is 5ms", avg)
	}
}
