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
	base := strings.TrimSuffix(filepath.Base(s.Command), ".exe")
	if base != "mcpgraph" || len(s.Args) == 0 || s.Args[0] != "proxy" {
		return s, false
	}
	for i, a := range s.Args {
		if a != "--" {
			continue
		}
		if i+1 >= len(s.Args) {
			return s, false // "proxy --" with nothing to run
		}
		inner := s
		inner.Command = s.Args[i+1]
		// Copy instead of re-slicing, so the caller's Args is never shared.
		inner.Args = append([]string(nil), s.Args[i+2:]...)
		return inner, true
	}
	return s, false
}
