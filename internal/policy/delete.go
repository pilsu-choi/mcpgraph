package policy

import (
	"path/filepath"
	"strings"
)

// recursiveDeleteOutside reports whether a segment is `rm` with -r/-R/
// --recursive and at least one target that is not strictly inside the
// project directory cwd. Deleting the project directory itself counts as
// outside.
//
// It errs on the side of asking: a target with an unexpanded variable
// ($DIR), or any target when cwd is unknown, counts as outside.
func recursiveDeleteOutside(s segment, home, cwd string) bool {
	if s.name() != "rm" {
		return false
	}
	recursive := false
	var targets []string
	endOfFlags := false
	for _, a := range s.args() {
		switch {
		case endOfFlags || !strings.HasPrefix(a, "-") || a == "-":
			targets = append(targets, a)
		case a == "--":
			endOfFlags = true
		case a == "--recursive":
			recursive = true
		case !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR"):
			recursive = true
		}
	}
	if !recursive {
		return false
	}
	for _, t := range targets {
		if !insideProject(t, home, cwd) {
			return true
		}
	}
	return false
}

// insideProject reports whether target resolves strictly below cwd.
func insideProject(target, home, cwd string) bool {
	if cwd == "" {
		return false
	}
	p := resolvePath(globBase(target), home, cwd)
	if strings.Contains(p, "$") || strings.HasPrefix(p, "~") {
		return false // unexpanded variable or another user's home (~bob)
	}
	root := filepath.Clean(cwd)
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, "../")
}

// globBase cuts a path at its first glob character, so "build/*" is judged
// as "build/" and "/*" as "/". A glob directly in the first segment
// ("*.log") keeps the directory it is in (".").
func globBase(p string) string {
	i := strings.IndexAny(p, "*?[")
	if i < 0 {
		return p
	}
	dir := p[:i]
	if j := strings.LastIndex(dir, "/"); j >= 0 {
		return dir[:j+1]
	}
	// "*.log" is inside the current directory: judge a file in it.
	return "./" + p
}
