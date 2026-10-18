package install

import (
	"encoding/json"
	"fmt"

	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
)

// entry is one MCP server entry found in a config file, with the spans
// install needs to rewrite it.
type entry struct {
	name    string
	where   string // human-readable location, e.g. `projects["/work/app"].mcpServers.slack`
	obj     *node  // the server object
	command *node  // nil if missing
	args    *node  // nil if missing
	// cmd and argv are the decoded command line.
	cmd  string
	argv []string
	// stdio is true when the app would start this server as a local
	// process (the only kind the proxy can wrap).
	stdio bool
}

// containers returns the objects that hold server entries in one app's
// config, keyed by a readable prefix. The shapes are the ones the config
// collector parses (see internal/collector/config/*.go); a change there
// must be mirrored here.
func containers(app string, root *node, src []byte) []struct {
	prefix string
	obj    *node
} {
	type c = struct {
		prefix string
		obj    *node
	}
	switch app {
	case "Claude Desktop", "Cursor":
		return []c{{"mcpServers", root.get("mcpServers")}}
	case "VS Code":
		return []c{{"servers", root.get("servers")}}
	case "Claude Code":
		out := []c{{"mcpServers", root.get("mcpServers")}}
		if projects := root.get("projects"); projects != nil && projects.kind == 'o' {
			for _, m := range projects.members {
				key, _ := json.Marshal(m.key)
				out = append(out, c{fmt.Sprintf("projects[%s].mcpServers", key), m.value.get("mcpServers")})
			}
		}
		return out
	}
	return nil
}

// findEntries lists every server entry in a config file, in file order.
func findEntries(app string, root *node, src []byte) []entry {
	var out []entry
	for _, c := range containers(app, root, src) {
		if c.obj == nil || c.obj.kind != 'o' {
			continue
		}
		for _, m := range c.obj.members {
			if m.value.kind != 'o' {
				continue
			}
			e := entry{name: m.key, where: c.prefix + "." + m.key, obj: m.value}
			e.command = m.value.get("command")
			e.args = m.value.get("args")
			e.cmd, _ = e.command.str(src)
			if e.args != nil && e.args.kind == 'a' {
				for _, a := range e.args.elems {
					if s, ok := a.str(src); ok {
						e.argv = append(e.argv, s)
					} else {
						// A number or bool in args: keep its JSON text,
						// as the config collector does.
						e.argv = append(e.argv, string(src[a.start:a.end]))
					}
				}
			}
			typ, _ := m.value.get("type").str(src)
			hasURL := m.value.get("url") != nil || m.value.get("serverUrl") != nil
			e.stdio = e.command != nil && e.cmd != "" && (typ == "stdio" || (typ == "" && !hasURL))
			out = append(out, e)
		}
	}
	return out
}

// proxied reports whether the entry already runs through mcpgraph proxy.
func (e entry) proxied() bool {
	_, _, ok := config.UnwrapProxyCommand(e.cmd, e.argv)
	return ok
}
