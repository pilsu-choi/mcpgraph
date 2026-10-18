package config

import "encoding/json"

// cursor parses Cursor's global mcp.json. It has the same shape as Claude
// Desktop's file, plus "url" entries for remote servers.
type cursor struct{}

func (cursor) App() string { return "Cursor" }

func (cursor) Parse(data []byte) ([]serverConfig, error) {
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return serversFromMap(doc.MCPServers, "$.mcpServers")
}
