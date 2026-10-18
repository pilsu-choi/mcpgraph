package config

import (
	"strconv"
	"strings"

	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/redact"
)

// deriver accumulates the nodes and edges produced from one config file.
// Nodes are deduplicated by ID because two servers may share a data source.
type deriver struct {
	home  string
	nodes []model.Node
	seen  map[string]bool
	edges []model.Edge
}

func newDeriver(home string) *deriver {
	return &deriver{home: home, seen: map[string]bool{}}
}

func (d *deriver) addNode(n model.Node) {
	if d.seen[n.ID] {
		return
	}
	d.seen[n.ID] = true
	d.nodes = append(d.nodes, n)
}

func (d *deriver) addEdge(from, to string, kind model.EdgeKind) {
	d.edges = append(d.edges, model.Edge{From: from, To: to, Kind: kind})
}

// app adds the AIApp node for a config file and returns its ID.
func (d *deriver) app(name, path string) string {
	id := model.NodeID(model.KindAIApp, name+"|"+path)
	d.addNode(model.Node{
		ID:    id,
		Kind:  model.KindAIApp,
		Label: name,
		App:   &model.AIApp{Name: name, ConfigPath: path},
	})
	return id
}

// server adds an MCPServer node plus its credentials and data sources.
// This is the only place raw config values are read; everything stored on
// a node goes through the redact package first.
func (d *deriver) server(appID, appName, path string, s serverConfig) {
	id := model.NodeID(model.KindMCPServer, path+"|"+s.JSONPath)
	d.addNode(model.Node{
		ID:    id,
		Kind:  model.KindMCPServer,
		Label: s.Name,
		Server: &model.MCPServer{
			Name:         s.Name,
			App:          appName,
			Command:      redact.Value("", s.Command),
			Args:         redact.Args(s.Args),
			EnvKeys:      sortedKeys(s.Env),
			Transport:    transport(s),
			URL:          redact.URL(s.URL),
			SourceConfig: path,
		},
	})
	d.addEdge(appID, id, model.EdgeConfigures)

	loc := path + "#" + s.JSONPath
	for _, k := range sortedKeys(s.Env) {
		d.credential(id, k, s.Env[k], loc+".env."+k)
	}
	for _, k := range sortedKeys(s.Headers) {
		d.credential(id, k, s.Headers[k], loc+".headers."+k)
	}
	if s.URL != "" && redact.URL(s.URL) != s.URL {
		d.addCredential(id, model.CredentialInline, "url", loc+".url", "password or token inside the server URL")
	}
	if s.EnvFile != "" {
		d.addCredential(id, model.CredentialFile, "envFile", loc+".envFile", "env file "+s.EnvFile)
	}
	d.argCredentials(id, s.Args, loc)

	for _, ds := range dataSources(s, d.home) {
		dsID := model.NodeID(model.KindDataSource, ds.Kind+"|"+ds.Identifier)
		d.addNode(model.Node{
			ID:         dsID,
			Kind:       model.KindDataSource,
			Label:      ds.Identifier,
			DataSource: &model.DataSource{Kind: ds.Kind, Identifier: ds.Identifier},
		})
		d.addEdge(id, dsID, model.EdgeAccesses)
	}
}

// credential classifies one key/value pair (env var or header) and records
// a Credential node if it is secret-related. The value itself is dropped.
func (d *deriver) credential(serverID, key, value, location string) {
	switch {
	case redact.IsReference(value):
		if redact.IsSecretKey(key) {
			d.addCredential(serverID, model.CredentialEnv, key, location, "value is read from the environment")
		}
	case redact.LooksLikePath(value) && (redact.IsSecretKey(key) || isCredentialPath(value)):
		d.addCredential(serverID, model.CredentialFile, key, location, "points to a credential file")
	default:
		if ok, why := redact.Detect(key, value); ok {
			d.addCredential(serverID, model.CredentialInline, key, location, why)
		}
	}
}

// argCredentials looks for secrets and credential file paths in args.
func (d *deriver) argCredentials(serverID string, args []string, loc string) {
	for i, a := range args {
		argLoc := loc + ".args[" + strconv.Itoa(i) + "]"
		key := "args[" + strconv.Itoa(i) + "]"
		value := a
		if strings.HasPrefix(a, "-") && strings.Contains(a, "=") {
			key, value, _ = strings.Cut(a, "=")
		} else if i > 0 && strings.HasPrefix(args[i-1], "-") && redact.IsSecretKey(args[i-1]) {
			key = args[i-1]
		}

		if redact.LooksLikePath(value) && isCredentialPath(value) {
			d.addCredential(serverID, model.CredentialFile, key, argLoc, "points to a credential file")
			continue
		}
		if redact.URL(value) != value && strings.Contains(value, "://") {
			d.addCredential(serverID, model.CredentialInline, key, argLoc, "password or token inside a URL")
			continue
		}
		if ok, why := redact.Detect(secretKeyOrEmpty(key), value); ok {
			d.addCredential(serverID, model.CredentialInline, key, argLoc, why)
		}
	}
}

// secretKeyOrEmpty keeps flag names (which may be secret-named) but drops
// the synthetic "args[3]" key so it does not influence detection.
func secretKeyOrEmpty(key string) string {
	if strings.HasPrefix(key, "args[") {
		return ""
	}
	return key
}

func (d *deriver) addCredential(serverID, kind, key, location, reason string) {
	id := model.NodeID(model.KindCredential, location)
	d.addNode(model.Node{
		ID:    id,
		Kind:  model.KindCredential,
		Label: key,
		Credential: &model.Credential{
			Kind:     kind,
			KeyName:  key,
			Location: location,
			Reason:   reason,
		},
	})
	d.addEdge(serverID, id, model.EdgeUsesSecret)
}

// transport normalises the many spellings of the transport type.
func transport(s serverConfig) string {
	switch strings.ToLower(s.Type) {
	case "sse":
		return model.TransportSSE
	case "http", "streamable-http", "streamablehttp":
		return model.TransportHTTP
	case "stdio":
		return model.TransportStdio
	}
	if s.URL != "" {
		return model.TransportHTTP
	}
	return model.TransportStdio
}

// isCredentialPath reports whether a path names a well-known credential file.
func isCredentialPath(p string) bool {
	lower := strings.ToLower(p)
	for _, s := range []string{".aws/credentials", ".ssh/id_", "/.env", ".pem", ".p12", ".key", "credentials.json", "service-account", ".netrc", ".pgpass"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return strings.HasSuffix(lower, ".env") || lower == ".env"
}
