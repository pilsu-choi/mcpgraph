package main

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRenderIsValidXML(t *testing.T) {
	lines := []string{
		"$ mcpgraph scan",
		"MCP SERVERS (2)",
		`node server.js --api-key ****FAKE <tag> & "quote"`,
		"HIGH (1)",
		"  [R1] Plaintext secret",
		`{"level":"INFO","msg":"new MCP server \"x\" added"}`,
	}
	svg := Render("t & <t>", lines)

	// Decode the whole document; any escaping mistake is a syntax error.
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		_, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("invalid XML: %v\n%s", err, svg)
		}
	}
	for _, want := range []string{"&lt;tag&gt;", "&amp;", `fill="#d2a8ff" font-weight="bold">****FAKE`, "--api-key&#160;", `fill="#ff7b72" font-weight="bold">[R1]`} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q", want)
		}
	}
}

func TestLongLinesAreTruncated(t *testing.T) {
	svg := Render("t", []string{strings.Repeat("x", 500)})
	if strings.Contains(svg, strings.Repeat("x", maxCols)) {
		t.Fatal("line was not truncated")
	}
}

func wellFormed(t *testing.T, svg string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		_, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("invalid XML: %v\n%s", err, svg)
		}
	}
}

func TestRenderAnimated(t *testing.T) {
	raw := []string{
		"$ echo <hi> & bye",
		"#@ 2",
		`output "quoted" <tag> & more`,
		"last line",
	}
	svg := RenderAnimated("t", raw)
	wellFormed(t, svg)

	if n := strings.Count(svg, `<text x="16"`); n != 3 { // 3 lines (the #@ line is not drawn)
		t.Errorf("drawn lines = %d, want 3", n)
	}
	for _, want := range []string{
		"@keyframes k0{", "@keyframes k2{", "animation-iteration-count:infinite",
		"clip-path:inset(0 100% 0 0)", // typed command line
		"opacity:1",                   // plain output line
		`style="animation-name:k1"`,
		"&lt;tag&gt;", "&amp;", "&quot;quoted&quot;",
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q", want)
		}
	}
	if strings.Contains(svg, "<script") || strings.Contains(svg, "#@") {
		t.Error("svg must have no script and no raw directive")
	}
}

func TestStaticHasNoAnimation(t *testing.T) {
	svg := Render("t", []string{"a", "b"})
	if strings.Contains(svg, "@keyframes") || strings.Contains(svg, "<style>") {
		t.Fatal("static mode must not carry animation")
	}
}
