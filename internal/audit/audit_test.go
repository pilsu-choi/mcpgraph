package audit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

var t0 = time.Date(2026, 10, 18, 7, 0, 0, 0, time.UTC)

func decision(i int, v model.Verdict) model.Decision {
	return model.Decision{
		Time: t0.Add(time.Duration(i) * time.Minute), Source: model.SourceHook, Agent: "claude-code", App: "Claude Code",
		Tool: "Bash", Summary: fmt.Sprintf("cmd %d", i), Verdict: v,
	}
}

func TestAppendAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", FileName)
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range []model.Verdict{model.VerdictAllow, model.VerdictDeny, model.VerdictAsk, model.VerdictDeny} {
		if err := l.Append(decision(i, v)); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode %o, want 600", perm)
	}

	res, err := ReadFile(path, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Decisions) != 4 || res.Decisions[0].Summary != "cmd 0" || res.Decisions[3].Verdict != model.VerdictDeny {
		t.Fatalf("read back %+v", res.Decisions)
	}

	res, _ = ReadFile(path, Filter{Verdict: model.VerdictDeny, Limit: 1})
	if len(res.Decisions) != 1 || res.Decisions[0].Summary != "cmd 3" {
		t.Fatalf("filter+limit = %+v, want only the newest deny", res.Decisions)
	}
}

func TestAppendOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	write := func(i int) []byte {
		l, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Append(decision(i, model.VerdictAllow)); err != nil {
			t.Fatal(err)
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first := write(0)
	second := write(1)
	if !bytes.HasPrefix(second, first) {
		t.Fatal("re-opening the log changed earlier bytes; it must only append")
	}
	if bytes.Count(second, []byte("\n")) != 2 {
		t.Fatalf("want 2 lines, got %q", second)
	}
}

func TestConcurrentAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	// Two Log values on the same file stand in for two processes.
	logs := make([]*Log, 2)
	for i := range logs {
		l, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		logs[i] = l
		defer l.Close()
	}
	var wg sync.WaitGroup
	const perWriter = 200
	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				if err := logs[w%2].Append(decision(w*perWriter+i, model.VerdictAllow)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	res, err := ReadFile(path, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Decisions) != 4*perWriter || res.Skipped != 0 {
		t.Fatalf("got %d decisions, %d skipped; want %d, 0", len(res.Decisions), res.Skipped, 4*perWriter)
	}
}

func TestReadSkipsBadLinesAndHandlesLongLines(t *testing.T) {
	long := decision(1, model.VerdictAllow)
	long.Reason = strings.Repeat("x", 100_000) // longer than bufio.Scanner's 64 KiB default
	var buf bytes.Buffer
	if err := writeLines(&buf, decision(0, model.VerdictAllow), long); err != nil {
		t.Fatal(err)
	}
	buf.WriteString("not json\n\n{\"time\":") // garbage, blank line, torn last line
	res, err := Read(&buf, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Decisions) != 2 || res.Skipped != 2 {
		t.Fatalf("got %d decisions, %d skipped; want 2, 2", len(res.Decisions), res.Skipped)
	}
}

func writeLines(buf *bytes.Buffer, ds ...model.Decision) error {
	for _, d := range ds {
		b, err := jsonLine(d)
		if err != nil {
			return err
		}
		buf.Write(b)
	}
	return nil
}

func TestMissingLogIsEmpty(t *testing.T) {
	res, err := ReadFile(filepath.Join(t.TempDir(), "nope.jsonl"), Filter{})
	if err != nil || len(res.Decisions) != 0 {
		t.Fatalf("got %+v, %v", res, err)
	}
}

// TestAppendRedacts is hard rule 2 for the audit log: even if a caller
// passes raw secrets in the summary or reason, they never reach the file.
func TestAppendRedacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d := decision(0, model.VerdictDeny)
	d.Summary = "PGPASSWORD=FakePassw0rdXyz psql postgres://u:FakePassw0rdXyz@h/db --token ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234"
	d.Reason = "blocked: Authorization: Bearer Zx9Qw8Er7Ty6Ui5Op4As3Df2FAKE"
	if err := l.Append(d); err != nil {
		t.Fatal(err)
	}
	l.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range testutil.FakeSecrets {
		if bytes.Contains(raw, []byte(s)) {
			t.Errorf("audit log contains raw secret %q: %s", s, raw)
		}
	}
}

func TestFilter(t *testing.T) {
	d := model.Decision{Time: t0, Source: model.SourceProxy, Agent: "Postgres", Tool: "query", RuleID: "block-destructive-sql", Verdict: model.VerdictDeny}
	tests := []struct {
		f    Filter
		want bool
	}{
		{Filter{}, true},
		{Filter{Since: t0}, true},
		{Filter{Since: t0.Add(time.Second)}, false},
		{Filter{Verdict: model.VerdictDeny, Source: model.SourceProxy}, true},
		{Filter{Verdict: model.VerdictAsk}, false},
		{Filter{Source: model.SourceHook}, false},
		{Filter{Agent: "postgres", Tool: "QUERY"}, true},
		{Filter{Agent: "slack"}, false},
		{Filter{RuleID: "block-destructive-sql"}, true},
		{Filter{RuleID: "block"}, false},
	}
	for i, tt := range tests {
		if got := tt.f.Match(d); got != tt.want {
			t.Errorf("case %d %+v: got %v, want %v", i, tt.f, got, tt.want)
		}
	}
}

func TestDecisionNodeIDIsStable(t *testing.T) {
	a, b := DecisionNode(decision(1, model.VerdictDeny)), DecisionNode(decision(1, model.VerdictDeny))
	c := DecisionNode(decision(2, model.VerdictDeny))
	if a.ID != b.ID || a.ID == c.ID || a.Kind != model.KindDecision || !strings.HasPrefix(a.ID, "Decision:") {
		t.Fatalf("ids %s %s %s", a.ID, b.ID, c.ID)
	}
}
