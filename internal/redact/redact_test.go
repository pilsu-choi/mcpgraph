package redact

import (
	"reflect"
	"strings"
	"testing"
)

func TestMask(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234", "****1234"},
		{"short", "****"},
		{"exactly11ch", "****"},
		{"exactly12chr", "****2chr"},
		{"", "****"},
	}
	for _, tt := range tests {
		got := Mask(tt.in)
		if got != tt.want {
			t.Errorf("Mask(%q) = %q, want %q", tt.in, got, tt.want)
		}
		// Hard rule 2: at most the last 4 characters are revealed.
		if len(got) > len("****")+4 {
			t.Errorf("Mask(%q) reveals too much: %q", tt.in, got)
		}
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  bool
	}{
		// positives
		{"github prefix", "", "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE", true},
		{"anthropic prefix", "", "sk-ant-FAKE0123456789abcdef", true},
		{"slack prefix", "", "xoxb-FAKE00000000-FAKEFAKE", true},
		{"aws key id", "", "AKIAFAKEFAKEFAKEFAKE", true},
		{"jwt", "", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.FAKE", true},
		{"secret-named key", "DB_PASSWORD", "hunter2", true},
		{"secret-named flag", "--api-key", "abc123", true},
		{"bearer header", "Authorization", "Bearer FAKEtoken1234", true},
		{"high entropy", "", "Zx9Qw8Er7Ty6Ui5Op4As3Df2", true},
		// negatives
		{"empty", "API_KEY", "", false},
		{"env reference", "GITHUB_TOKEN", "${GITHUB_TOKEN}", false},
		{"vscode input", "Authorization", "Bearer ${input:token}", false},
		{"shell reference", "TOKEN", "$TOKEN", false},
		{"package name", "", "@modelcontextprotocol/server-filesystem", false},
		{"pinned package", "", "mcp-server-sqlite==0.6.2", false},
		{"plain word", "", "filesystem", false},
		{"path", "", "/home/user/projects/a1b2c3d4e5f6g7h8", false},
		{"bool under secret key", "USE_AUTH", "true", false},
		{"path under secret key", "GOOGLE_CREDENTIALS", "/etc/key.json", false},
		{"uuid", "", "123e4567-e89b-12d3-a456-426614174000", false},
		{"team id", "SLACK_TEAM_ID", "T01234567", false},
		{"short prefix match", "", "sk-1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, why := Detect(tt.key, tt.value)
			if got != tt.want {
				t.Errorf("Detect(%q, %q) = %v (%s), want %v", tt.key, tt.value, got, why, tt.want)
			}
			if got && why == "" {
				t.Error("a positive detection must explain why")
			}
		})
	}
}

func TestArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nothing secret",
			in:   []string{"-y", "@modelcontextprotocol/server-filesystem", "/home/u"},
			want: []string{"-y", "@modelcontextprotocol/server-filesystem", "/home/u"},
		},
		{
			name: "flag followed by value",
			in:   []string{"server.js", "--api-key", "sk-ant-FAKE0123456789abcdefFAKE"},
			want: []string{"server.js", "--api-key", "****FAKE"},
		},
		{
			name: "flag followed by short value",
			in:   []string{"--token", "abc"},
			want: []string{"--token", "****"},
		},
		{
			name: "flag=value",
			in:   []string{"--token=ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE"},
			want: []string{"--token=****FAKE"},
		},
		{
			name: "secret flag followed by reference",
			in:   []string{"--token", "${GITHUB_TOKEN}"},
			want: []string{"--token", "${GITHUB_TOKEN}"},
		},
		{
			name: "connection string",
			in:   []string{"postgresql://admin:FakePassw0rdXyz@db:5432/prod"},
			want: []string{"postgresql://admin:****@db:5432/prod"},
		},
		{
			name: "positional token",
			in:   []string{"run", "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE"},
			want: []string{"run", "****FAKE"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Args(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Args(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestArgsDoesNotModifyInput(t *testing.T) {
	in := []string{"--token", "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE"}
	_ = Args(in)
	if in[1] != "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE" {
		t.Error("Args modified its input slice")
	}
}

func TestURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://mcp.example.com/mcp", "https://mcp.example.com/mcp"},
		{"http://10.0.0.5:8080/mcp?api_key=FAKEqueryKey1234567890abcd", "http://10.0.0.5:8080/mcp?api_key=****abcd"},
		{"http://localhost:3000/mcp?debug=true", "http://localhost:3000/mcp?debug=true"},
		{"https://user:FakePassw0rdXyz@host/x", "https://user:****@host/x"},
		{"https://ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKE@github.com/x", "https://****@github.com/x"},
	}
	for _, tt := range tests {
		if got := URL(tt.in); got != tt.want {
			t.Errorf("URL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCmdline(t *testing.T) {
	args, line := Cmdline([]string{"node", "srv.js", "--api-key", "sk-ant-FAKE0123456789abcdefFAKE"})
	if strings.Contains(line, "sk-ant-FAKE0123456789abcdef") || args[3] != "****FAKE" {
		t.Errorf("Cmdline leaked: %q", line)
	}
}

func TestIsSecretKey(t *testing.T) {
	for _, k := range []string{"GITHUB_TOKEN", "--api-key", "Authorization", "client_secret", "PGPASSWORD"} {
		if !IsSecretKey(k) {
			t.Errorf("IsSecretKey(%q) = false", k)
		}
	}
	for _, k := range []string{"PORT", "--repository", "LOG_LEVEL", "SLACK_TEAM_ID"} {
		if IsSecretKey(k) {
			t.Errorf("IsSecretKey(%q) = true", k)
		}
	}
}
