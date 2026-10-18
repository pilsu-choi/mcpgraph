package config

import "encoding/json"

// claudeDesktop parses claude_desktop_config.json:
//
//	{"mcpServers": {"name": {"command": "...", "args": [...], "env": {...}}}}
type claudeDesktop struct{}

func (claudeDesktop) App() string { return "Claude Desktop" }

func (claudeDesktop) Parse(data []byte) ([]serverConfig, error) {
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return serversFromMap(doc.MCPServers, "$.mcpServers")
}
