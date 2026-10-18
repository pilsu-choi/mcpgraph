package config

import "path/filepath"

// Candidate is a config file location that may exist on this machine.
type Candidate struct {
	App  string
	Path string
	// Verified is true only when the location has been confirmed on a real
	// machine. Unverified paths are still scanned; they are candidates.
	Verified bool
	parser   parser
}

// Candidates returns every config location to check for the given home
// directory and GOOS ("darwin" or "linux").
//
// Verification log (see AGENTS.md hard rule 5):
//   - ~/.claude.json: confirmed on the owner's Linux (WSL2) machine on
//     2026-10-18; the file has top-level "mcpServers" and
//     "projects[...].mcpServers".
//   - everything else: not yet confirmed, marked TODO(verify).
func Candidates(home, goos string) []Candidate {
	var out []Candidate
	add := func(app, path string, verified bool, p parser) {
		out = append(out, Candidate{App: app, Path: path, Verified: verified, parser: p})
	}

	switch goos {
	case "darwin":
		appSupport := filepath.Join(home, "Library", "Application Support")
		// TODO(verify): Claude Desktop on macOS.
		add("Claude Desktop", filepath.Join(appSupport, "Claude", "claude_desktop_config.json"), false, claudeDesktop{})
		// TODO(verify): same home-relative path as Linux, not yet checked on a Mac.
		add("Claude Code", filepath.Join(home, ".claude.json"), false, claudeCode{})
		// TODO(verify): Cursor global MCP config on macOS.
		add("Cursor", filepath.Join(home, ".cursor", "mcp.json"), false, cursor{})
		// TODO(verify): VS Code user-level mcp.json on macOS.
		add("VS Code", filepath.Join(appSupport, "Code", "User", "mcp.json"), false, vscode{})

	case "linux":
		// TODO(verify): Claude Desktop has no official Linux build; this is
		// the path used by community builds.
		add("Claude Desktop", filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), false, claudeDesktop{})
		// Verified 2026-10-18 (see above).
		add("Claude Code", filepath.Join(home, ".claude.json"), true, claudeCode{})
		// TODO(verify): ~/.cursor exists on the owner's machine but has no
		// mcp.json yet, so the file name is unconfirmed.
		add("Cursor", filepath.Join(home, ".cursor", "mcp.json"), false, cursor{})
		// TODO(verify): VS Code user-level mcp.json on desktop Linux.
		add("VS Code", filepath.Join(home, ".config", "Code", "User", "mcp.json"), false, vscode{})
		// TODO(verify): VS Code Remote/WSL server; ~/.vscode-server/data/User
		// exists on the owner's machine but has no mcp.json yet.
		add("VS Code", filepath.Join(home, ".vscode-server", "data", "User", "mcp.json"), false, vscode{})
	}
	return out
}
