package config

import (
	"encoding/json"
	"fmt"
	"sort"
)

// claudeCode parses ~/.claude.json. Servers live in two places:
//
//	{"mcpServers": {...},                              // user scope
//	 "projects": {"/path/to/repo": {"mcpServers": {...}}}} // local (per-project) scope
//
// The file also holds lots of unrelated state; we decode only what we need.
type claudeCode struct{}

func (claudeCode) App() string { return "Claude Code" }

func (claudeCode) Parse(data []byte) ([]serverConfig, error) {
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
		Projects   map[string]struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	out, err := serversFromMap(doc.MCPServers, "$.mcpServers")
	if err != nil {
		return nil, err
	}

	projects := make([]string, 0, len(doc.Projects))
	for p := range doc.Projects {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	for _, p := range projects {
		prefix := fmt.Sprintf("$.projects[%q].mcpServers", p)
		servers, err := serversFromMap(doc.Projects[p].MCPServers, prefix)
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", p, err)
		}
		out = append(out, servers...)
	}
	return out, nil
}
