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
