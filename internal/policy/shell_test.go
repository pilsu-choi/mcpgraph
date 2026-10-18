package policy

import (
	"slices"
	"testing"
)

func TestSplitShell(t *testing.T) {
	tests := []struct {
		in   string
		want [][]string // words per segment
		cmds []string   // commandLine per segment
	}{
		{`ls -la`, [][]string{{"ls", "-la"}}, []string{"ls -la"}},
		{`a && b || c; d | e & f`, [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}}, []string{"a", "b", "c", "d", "e", "f"}},
		{`psql -c "DROP TABLE a; DROP TABLE b"`, [][]string{{"psql", "-c", "DROP TABLE a; DROP TABLE b"}}, []string{"psql -c DROP TABLE a; DROP TABLE b"}},
		{`echo 'a && b' "c \"q\" \$x"`, [][]string{{"echo", "a && b", `c "q" $x`}}, []string{`echo a && b c "q" $x`}},
		{`echo a\ b`, [][]string{{"echo", "a b"}}, []string{"echo a b"}},
		{`echo "" x`, [][]string{{"echo", "", "x"}}, []string{"echo  x"}},
		{"ls # rm -rf /\npwd", [][]string{{"ls"}, {"pwd"}}, []string{"ls", "pwd"}},
		{`cmd 2>&1 >/dev/null`, [][]string{{"cmd", "2>&1", ">/dev/null"}}, []string{"cmd 2>&1 >/dev/null"}},
		{`FOO=1 sudo -u pg /usr/bin/psql x`, [][]string{{"FOO=1", "sudo", "-u", "pg", "/usr/bin/psql", "x"}}, []string{"psql x"}},
		{`bash -lc "git push -f"`, [][]string{{"bash", "-lc", "git push -f"}, {"git", "push", "-f"}}, []string{"bash -lc git push -f", "git push -f"}},
		{`X=1`, [][]string{{"X=1"}}, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			segs := splitShell(tt.in)
			if len(segs) != len(tt.want) {
				t.Fatalf("got %d segments %+v, want %d", len(segs), segs, len(tt.want))
			}
			for i, s := range segs {
				if !slices.Equal(s.Words, tt.want[i]) {
					t.Errorf("segment %d words = %q, want %q", i, s.Words, tt.want[i])
				}
				if got := s.commandLine(); got != tt.cmds[i] {
					t.Errorf("segment %d commandLine = %q, want %q", i, got, tt.cmds[i])
				}
			}
		})
	}
}

func TestShellNestingIsBounded(t *testing.T) {
	// Deeper than maxShellDepth: must terminate and not explode.
	in := `sh -c "sh -c 'sh -c \"sh -c ls\"'"`
	if n := len(splitShell(in)); n == 0 || n > maxShellDepth+1 {
		t.Fatalf("got %d segments", n)
	}
}

func TestGlob(t *testing.T) {
	tests := []struct {
		pattern, path string
		want          bool
	}{
		{"~/.ssh/**", "/home/u/.ssh/id_rsa", true},
		{"~/.ssh/**", "/home/u/.ssh", true},
		{"~/.ssh/**", "/home/u/.sshx/a", false},
		{"**/.env", ".env", true},
		{"**/.env", "/a/b/.env", true},
		{"**/.env", "/a/b/.envrc", false},
		{"**/.env.*", "/a/.env.local", true},
		{"/etc/*.conf", "/etc/a.conf", true},
		{"/etc/*.conf", "/etc/x/a.conf", false},
		{"/abs/**", "abs/x", false}, // absolute pattern never matches a relative path
	}
	for _, tt := range tests {
		g, err := compileGlob(tt.pattern, "/home/u")
		if err != nil {
			t.Fatal(err)
		}
		if got := g.match(tt.path); got != tt.want {
			t.Errorf("%q vs %q = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestTautology(t *testing.T) {
	for cond, want := range map[string]bool{
		"1=1": true, " ( 1 = 1 ) ": true, "TRUE": true, "'a'='a'": true, "x = x": true, "1": true,
		"id = 1": false, "a = b": false, "deleted_at is null": false, "": false,
	} {
		if got := tautology(cond); got != want {
			t.Errorf("tautology(%q) = %v, want %v", cond, got, want)
		}
	}
}
