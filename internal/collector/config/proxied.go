package config

import (
	"path/filepath"
	"strings"
)

// unwrapProxy recognises a server entry that `mcpgraph install` rewrote to
// run through the proxy:
//
//	"command": "/usr/local/bin/mcpgraph",
//	"args": ["proxy", <proxy flags...>, "--", "npx", "-y", "pkg"]
//
// It returns the entry with Command and Args replaced by the wrapped server
// command, and true. Any other entry is returned unchanged with false.
//
// This is mcpgraph's own format (milestone 8 writes it), so it needs no
// external verification.
func unwrapProxy(s serverConfig) (serverConfig, bool) {
	cmd, args, ok := UnwrapProxyCommand(s.Command, s.Args)
	if !ok {
		return s, false
	}
	inner := s
	inner.Command = cmd
	inner.Args = args
	return inner, true
}

// UnwrapProxyCommand is unwrapProxy for a bare command line: if command and
// args are an `mcpgraph proxy ... -- cmd args...` wrapper, it returns the
// wrapped cmd and args, and true. install and uninstall use it too, so
// all three agree on what "proxied" means.
func UnwrapProxyCommand(command string, args []string) (string, []string, bool) {
	base := strings.TrimSuffix(filepath.Base(command), ".exe")
	if base != "mcpgraph" || len(args) == 0 || args[0] != "proxy" {
		return command, args, false
	}
	for i, a := range args {
		if a != "--" {
			continue
		}
		if i+1 >= len(args) {
			return command, args, false // "proxy --" with nothing to run
		}
		// Copy instead of re-slicing, so the caller's slice is never shared.
		return args[i+1], append([]string(nil), args[i+2:]...), true
	}
	return command, args, false
}
