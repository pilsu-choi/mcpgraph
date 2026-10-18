package redact

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		secret string // must not appear in the output ("" = nothing to hide)
		want   string // substring that must survive
	}{
		{"plain command kept", `psql -c "DROP TABLE users"`, "", `psql -c "DROP TABLE users"`},
		{"env assignment", `PGPASSWORD=FakePassw0rdXyz psql -h db`, "FakePassw0rdXyz", "PGPASSWORD=****"},
		{"url password", `psql postgres://app:FakePassw0rdXyz@db:5432/prod -c "DELETE FROM t"`, "FakePassw0rdXyz", "DELETE FROM t"},
		{"bearer header", `curl -H "Authorization: Bearer Zx9Qw8Er7Ty6Ui5Op4As3Df2FAKE" https://x`, "Zx9Qw8Er7Ty6Ui5Op4As3Df2FAKE", "curl -H"},
		{"secret flag", `server --api-key sk-ant-FAKE0123456789abcdefFAKE`, "sk-ant-FAKE0123456789abcdefFAKE", "--api-key ****FAKE"},
		{"quoted flag value", `run "--token=ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234"`, "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234", "--token=****"},
		{"sql password", `ALTER USER app PASSWORD 'FakePassw0rdXyz';`, "FakePassw0rdXyz", "ALTER USER app PASSWORD"},
		{"known prefix anywhere", `echo xoxb-FAKE00000000-FAKEFAKEFAKEFAKE | slack`, "xoxb-FAKE00000000-FAKEFAKEFAKEFAKE", "| slack"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Text(tt.in)
			if tt.secret != "" && strings.Contains(got, tt.secret) {
				t.Errorf("Text(%q) = %q leaks the secret", tt.in, got)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("Text(%q) = %q, want it to contain %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsEnvAssignment(t *testing.T) {
	for w, want := range map[string]bool{
		"A=1": true, "PG_PASS=x": true, "_X1=": true,
		"1A=x": false, "--flag=x": false, "=x": false, "a-b=x": false, "plain": false,
	} {
		if got := isEnvAssignment(w); got != want {
			t.Errorf("isEnvAssignment(%q) = %v, want %v", w, got, want)
		}
	}
}
