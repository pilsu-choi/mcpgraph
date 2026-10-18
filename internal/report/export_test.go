package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func sampleGraph() *model.Graph {
	return &model.Graph{
		Nodes: []model.Node{
			{ID: "AIApp:aaa", Kind: model.KindAIApp, Label: "Cursor", App: &model.AIApp{Name: "Cursor"}},
			{ID: "MCPServer:bbb", Kind: model.KindMCPServer, Label: `say "hi" <b>#1`,
				Server: &model.MCPServer{Name: `say "hi" <b>#1`, App: "Cursor", Transport: "stdio"}},
		},
		Edges: []model.Edge{{From: "AIApp:aaa", To: "MCPServer:bbb", Kind: model.EdgeConfigures}},
	}
}

func TestMermaid(t *testing.T) {
	var buf bytes.Buffer
	if err := Mermaid(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"flowchart LR\n",
		`n_AIApp_aaa["AIApp<br/>Cursor"]:::AIApp`,
		`n_AIApp_aaa -->|CONFIGURES| n_MCPServer_bbb`,
		`say #quot;hi#quot; #lt;b#gt;#35;1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mermaid output missing %q\n%s", want, out)
		}
	}
	// Raw quotes inside a label would end the label early.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "[\"") && strings.Count(line, `"`) != 2 {
			t.Errorf("unescaped quote in %q", line)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	var back model.Graph
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Nodes) != 2 || back.Nodes[1].Server.Name != `say "hi" <b>#1` {
		t.Fatalf("round trip lost data: %+v", back)
	}
}
