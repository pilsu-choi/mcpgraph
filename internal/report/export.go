package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// JSON writes v as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Mermaid writes the graph as a Mermaid flowchart, which GitHub renders
// inside a ```mermaid code block.
func Mermaid(w io.Writer, g *model.Graph) error {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for _, kind := range []model.NodeKind{model.KindHost, model.KindAIApp, model.KindMCPServer, model.KindProcess, model.KindCredential, model.KindDataSource} {
		fmt.Fprintf(&b, "  classDef %s %s\n", kind, mermaidStyle[kind])
	}
	for _, n := range g.Nodes {
		fmt.Fprintf(&b, "  %s[\"%s\"]:::%s\n", mermaidID(n.ID), mermaidText(nodeTitle(n)), n.Kind)
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&b, "  %s -->|%s| %s\n", mermaidID(e.From), e.Kind, mermaidID(e.To))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

var mermaidStyle = map[model.NodeKind]string{
	model.KindHost:       "fill:#eeeeee,stroke:#666666,color:#000000",
	model.KindAIApp:      "fill:#dbeafe,stroke:#1d4ed8,color:#000000",
	model.KindMCPServer:  "fill:#ede9fe,stroke:#6d28d9,color:#000000",
	model.KindProcess:    "fill:#dcfce7,stroke:#15803d,color:#000000",
	model.KindCredential: "fill:#fee2e2,stroke:#b91c1c,color:#000000",
	model.KindDataSource: "fill:#fef3c7,stroke:#b45309,color:#000000",
}

// nodeTitle is the two-line text shown in a Mermaid box.
func nodeTitle(n model.Node) string {
	detail := n.Label
	switch n.Kind {
	case model.KindMCPServer:
		detail = n.Server.App + " / " + n.Server.Name
	case model.KindCredential:
		detail = n.Credential.Kind + ": " + n.Credential.KeyName
	case model.KindDataSource:
		detail = n.DataSource.Kind + ": " + n.DataSource.Identifier
	}
	return string(n.Kind) + "\n" + truncate(detail, 60)
}

// mermaidID turns "MCPServer:ab12..." into a safe identifier.
func mermaidID(id string) string {
	return "n_" + strings.NewReplacer(":", "_", "-", "_").Replace(id)
}

// mermaidText escapes characters that would end or break a quoted label.
// Mermaid uses #quot; style entity codes.
func mermaidText(s string) string {
	// NewReplacer makes a single pass, so the "#" inserted by later rules
	// is not escaped again.
	r := strings.NewReplacer(
		"#", "#35;",
		`"`, "#quot;",
		"<", "#lt;",
		">", "#gt;",
		"\n", "<br/>",
	)
	return r.Replace(s)
}
