package install

import (
	"strings"
	"testing"
)

func TestParseJSONSpans(t *testing.T) {
	src := []byte(`// comment
{
  "a": [1, "two", {"b": null},],  /* trailing comma */
  "s": "q\"uote // not a comment",
  "a": true,
}`)
	root, err := parseJSON(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := root.get("a"); got == nil || string(src[got.start:got.end]) != "true" {
		t.Fatalf("duplicate key: last one should win, got %v", got)
	}
	s, ok := root.get("s").str(src)
	if !ok || s != `q"uote // not a comment` {
		t.Fatalf("string = %q", s)
	}
	arr := root.members[0].value
	if arr.kind != 'a' || len(arr.elems) != 3 || string(src[arr.elems[1].start:arr.elems[1].end]) != `"two"` {
		t.Fatalf("array = %+v", arr)
	}
}

func TestParseJSONErrors(t *testing.T) {
	for _, src := range []string{
		``, `{`, `{"a" 1}`, `{"a":}`, `{"a":1 "b":2}`, `[1 2]`, `"unterminated`, `{"a":tru}`,
		`{"a":1} extra`, `{"bad\escape":1}`, strings.Repeat("[", 600) + strings.Repeat("]", 600),
	} {
		if _, err := parseJSON([]byte(src)); err == nil {
			t.Errorf("parseJSON(%.20q) succeeded, want an error", src)
		}
	}
}

func TestRedactJSONLine(t *testing.T) {
	tests := []struct{ in, want string }{
		{`    "TOKEN": "ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234",`, `    "TOKEN": "****1234",`},
		{`  "args": ["--api-key", "sk-ant-FAKE0123456789abcdefFAKE"]`, `  "args": ["--api-key", "****FAKE"]`},
		{`  "postgresql://admin:FakePassw0rdXyz@db/prod"`, `  "postgresql://admin:****@db/prod"`},
		{`  "command": "npx",`, `  "command": "npx",`},
		{`  },`, `  },`},
	}
	for _, tt := range tests {
		if got := redactJSONLine(tt.in); got != tt.want {
			t.Errorf("redactJSONLine(%q)\n got %q\nwant %q", tt.in, got, tt.want)
		}
	}
}
