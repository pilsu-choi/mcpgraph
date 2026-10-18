package config

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/netutil"
)

// pathFlags are flags whose value is a directory the server can access.
var pathFlags = []string{"--root", "--roots", "--dir", "--directory", "--path", "--repository", "--repo", "--workspace", "--allowed-dir", "--allowed-directories"}

// databaseSchemes are URL schemes of database connection strings.
var databaseSchemes = []string{"postgres", "postgresql", "mysql", "mariadb", "mongodb", "mongodb+srv", "redis", "rediss", "sqlserver", "clickhouse"}

// saasKeywords maps a substring of a server's name, command or args to the
// service it talks to. The list is short on purpose: unknown servers simply
// get no SaaS data source.
var saasKeywords = []struct{ keyword, service string }{
	{"github", "github"},
	{"gitlab", "gitlab"},
	{"slack", "slack"},
	{"notion", "notion"},
	{"linear", "linear"},
	{"atlassian", "atlassian"},
	{"jira", "atlassian"},
	{"confluence", "atlassian"},
	{"gdrive", "google-drive"},
	{"google-drive", "google-drive"},
	{"sentry", "sentry"},
	{"stripe", "stripe"},
	{"figma", "figma"},
	{"supabase", "supabase"},
}

// dataSources infers what a server can reach from its command, args, env
// and URL. These are heuristics; the README lists their limits.
func dataSources(s serverConfig, home string) []model.DataSource {
	var out []model.DataSource
	add := func(kind, id string) {
		for _, ds := range out {
			if ds.Kind == kind && ds.Identifier == id {
				return
			}
		}
		out = append(out, model.DataSource{Kind: kind, Identifier: id})
	}

	for _, p := range filesystemRoots(s, home) {
		add(model.DataSourceFilesystem, p)
	}

	values := append([]string{}, s.Args...)
	for _, k := range sortedKeys(s.Env) {
		values = append(values, s.Env[k])
	}
	for _, v := range values {
		if id, ok := databaseIdentifier(v); ok {
			add(model.DataSourceDatabase, id)
		}
	}

	haystack := strings.ToLower(s.Name + " " + s.Command + " " + strings.Join(s.Args, " ") + " " + s.URL)
	for _, kw := range saasKeywords {
		if strings.Contains(haystack, kw.keyword) {
			add(model.DataSourceSaaS, kw.service)
		}
	}
	// A remote server is itself a SaaS endpoint.
	if u, err := url.Parse(s.URL); err == nil && u.Host != "" && !netutil.IsLoopbackHost(u.Hostname()) {
		add(model.DataSourceSaaS, u.Hostname())
	}
	return out
}

// filesystemRoots returns the absolute directories a server is given.
//
// For a filesystem server (name or args mention "filesystem") every path
// argument is a root, which is how @modelcontextprotocol/server-filesystem
// works. For any other server only values of path flags such as --root
// count, so a script path like "node /opt/server.js" is not mistaken for
// data access.
func filesystemRoots(s serverConfig, home string) []string {
	isFS := strings.Contains(strings.ToLower(s.Name+" "+strings.Join(s.Args, " ")), "filesystem")

	var roots []string
	for i, a := range s.Args {
		// strings.Cut splits "--root=/x" into "--root" and "/x"; found is
		// false when there is no "=".
		flag, value, found := strings.Cut(a, "=")
		var p string
		switch {
		case isFS && looksLikeDir(a):
			p = a
		case found && isPathFlag(flag):
			p = value
		case i > 0 && isPathFlag(s.Args[i-1]) && looksLikeDir(a):
			p = a
		default:
			continue
		}
		roots = append(roots, expandHome(p, home))
	}
	return roots
}

func isPathFlag(f string) bool {
	for _, pf := range pathFlags {
		if f == pf {
			return true
		}
	}
	return false
}

func looksLikeDir(a string) bool {
	return a == "/" || a == "~" || a == "." || strings.HasPrefix(a, "/") || strings.HasPrefix(a, "~/") || strings.HasPrefix(a, "./")
}

// expandHome turns "~" and "~/x" into absolute paths under home and cleans
// the result, so "/home/u/" and "~" compare equal.
func expandHome(p, home string) string {
	if p == "~" {
		return filepath.Clean(home)
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return filepath.Clean(p)
}

// databaseIdentifier returns "scheme://host[:port]/db" for a database
// connection string, with user and password removed.
func databaseIdentifier(v string) (string, bool) {
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", false
	}
	for _, s := range databaseSchemes {
		if u.Scheme == s {
			return u.Scheme + "://" + u.Host + u.Path, true
		}
	}
	return "", false
}
