// Command fakemcp is a tiny scripted stdio MCP server for docs and demos.
// It speaks just enough of MCP 2025-11-25 to show `mcpgraph proxy` at
// work: initialize, tools/list (tools "query" and "read_file"), tools/call
// (echoes what it would have run) and an empty result for anything else.
// It never runs anything.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Fprintln(os.Stderr, "fakemcp: ready") // MCP servers may log on stderr
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	for {
		line, err := in.ReadBytes('\n')
		if len(line) > 0 {
			if reply := respond(line); reply != "" {
				// One message per line, sent right away. A write error means
				// the client is gone; the next read will end the loop.
				_, _ = out.WriteString(reply + "\n")
				_ = out.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func respond(line []byte) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(line, &m) != nil {
		return `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"parse error"}}`
	}
	var method string
	_ = json.Unmarshal(m["method"], &method)
	id, ok := m["id"]
	if method == "" || !ok {
		return "" // notification, or a response to us
	}
	result := `{}`
	switch method {
	case "initialize":
		result = `{"protocolVersion":"2025-11-25","capabilities":{"tools":{}},"serverInfo":{"name":"fakemcp","version":"1.0.0"}}`
	case "tools/list":
		result = `{"tools":[{"name":"query","inputSchema":{"type":"object"}},{"name":"read_file","inputSchema":{"type":"object"}}]}`
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(m["params"], &p)
		var vals []string
		for _, v := range p.Arguments {
			vals = append(vals, fmt.Sprint(v))
		}
		text, _ := json.Marshal("fakemcp would run " + p.Name + ": " + strings.Join(vals, " "))
		result = `{"content":[{"type":"text","text":` + string(text) + `}]}`
	}
	return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":` + result + `}`
}
