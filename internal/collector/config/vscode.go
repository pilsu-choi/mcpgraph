package config

import "encoding/json"

// vscode parses VS Code's user-level mcp.json:
//
//	{"servers": {"name": {"type": "stdio", "command": "...", ...}},
//	 "inputs": [...]}
//
// VS Code files are JSONC (comments and trailing commas allowed), so we
// strip those first.
type vscode struct{}

func (vscode) App() string { return "VS Code" }

func (vscode) Parse(data []byte) ([]serverConfig, error) {
	var doc struct {
		Servers map[string]json.RawMessage `json:"servers"`
	}
	if err := json.Unmarshal(StripJSONC(data), &doc); err != nil {
		return nil, err
	}
	return serversFromMap(doc.Servers, "$.servers")
}
