package config

import (
	"encoding/json"
	"fmt"
	"sort"
)

// serverConfig is one MCP server entry exactly as written in a config file.
//
// It can contain secret values (env values, headers, args), so the type is
// unexported: nothing outside this package can hold one. derive.go turns it
// into redacted graph nodes before anything is sent to the graph builder.
type serverConfig struct {
	Name string
	// JSONPath locates the entry inside the file, e.g. `$.mcpServers.github`.
	JSONPath string
	Type     string
	Command  string
	Args     []string
	Env      map[string]string
	Headers  map[string]string
	EnvFile  string
	URL      string
}

// rawServer mirrors the JSON shape shared by all four supported apps.
// Fields an app does not use simply stay empty.
type rawServer struct {
	Type    string         `json:"type"`
	Command string         `json:"command"`
	Args    []any          `json:"args"`
	Env     map[string]any `json:"env"`
	Headers map[string]any `json:"headers"`
	EnvFile string         `json:"envFile"`
	URL     string         `json:"url"`
	// Some clients use serverUrl for remote servers.
	ServerURL string `json:"serverUrl"`
}

// parser turns the bytes of one app's config file into server entries.
//
// Any type with these two methods satisfies the interface automatically;
// Go has no "implements" keyword.
type parser interface {
	App() string
	Parse(data []byte) ([]serverConfig, error)
}

// serversFromMap converts a {"name": {...}} JSON object into server entries,
// sorted by name so output is deterministic. pathPrefix is the JSON path of
// the object itself, e.g. `$.mcpServers`.
func serversFromMap(m map[string]json.RawMessage, pathPrefix string) ([]serverConfig, error) {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	// Map iteration order in Go is random on purpose, so sort the keys.
	sort.Strings(names)

	var out []serverConfig
	for _, name := range names {
		var r rawServer
		if err := json.Unmarshal(m[name], &r); err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}
		url := r.URL
		if url == "" {
			url = r.ServerURL
		}
		out = append(out, serverConfig{
			Name:     name,
			JSONPath: pathPrefix + "." + name,
			Type:     r.Type,
			Command:  r.Command,
			Args:     toStrings(r.Args),
			Env:      toStringMap(r.Env),
			Headers:  toStringMap(r.Headers),
			EnvFile:  r.EnvFile,
			URL:      url,
		})
	}
	return out, nil
}

// toStrings converts JSON values to strings. Configs sometimes contain
// numbers in args (e.g. a port), which would fail a strict []string decode.
func toStrings(in []any) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, fmt.Sprint(v))
	}
	return out
}

func toStringMap(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = fmt.Sprint(v)
	}
	return out
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
