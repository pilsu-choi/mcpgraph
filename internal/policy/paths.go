package policy

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// redirection matches a leading shell redirection operator such as <, >>,
// 2> or >&, so "<~/.ssh/id_rsa" yields the path.
var redirection = regexp.MustCompile(`^[0-9]*[<>]+&?`)

// globPattern is a compiled path pattern from the policy, such as
// "~/.ssh/**" or "**/.env". "~" is expanded at load time.
//
// Supported syntax: "**" matches any number of path segments (including
// none); inside a segment, path.Match syntax (*, ?, [...]) applies.
type globPattern struct {
	raw      string
	segments []string
}

func compileGlob(pattern, home string) (globPattern, error) {
	p := expandHome(pattern, home)
	segs := splitPath(p)
	for _, s := range segs {
		if s == "**" {
			continue
		}
		// path.Match reports a malformed pattern only when it is used,
		// so try it once now to fail at load time instead.
		if _, err := path.Match(s, ""); err != nil {
			return globPattern{}, err
		}
	}
	return globPattern{raw: pattern, segments: segs}, nil
}

// match reports whether the cleaned path p matches the pattern.
func (g globPattern) match(p string) bool {
	return matchSegments(g.segments, splitPath(p))
}

func matchSegments(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// Try to let ** absorb 0, 1, 2, ... segments.
			for skip := 0; skip <= len(segs); skip++ {
				if matchSegments(pat[1:], segs[skip:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], segs[0]); !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}

// splitPath splits "/a/b" into ["", "a", "b"] and "a/b" into ["a", "b"],
// so an absolute pattern never matches a relative path by accident.
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// expandHome replaces a leading ~, $HOME or ${HOME} with home.
func expandHome(p, home string) string {
	if home == "" {
		return p
	}
	for _, prefix := range []string{"~", "$HOME", "${HOME}"} {
		if p == prefix {
			return home
		}
		if strings.HasPrefix(p, prefix+"/") {
			return home + p[len(prefix):]
		}
	}
	return p
}

// resolvePath turns a word from a command into a cleaned path: it expands
// the home directory and makes relative paths absolute against cwd. When
// cwd is unknown a relative path stays relative, which "**/" patterns still
// match.
func resolvePath(word, home, cwd string) string {
	p := expandHome(word, home)
	if !filepath.IsAbs(p) && cwd != "" && !strings.HasPrefix(p, "~") && !strings.HasPrefix(p, "$") {
		p = filepath.Join(cwd, p)
	}
	return filepath.Clean(p)
}

// pathCandidates pulls words that may be file paths out of shell segments
// and argument values: anything containing "/" or starting with "." or "~",
// the value part of "--flag=value", and redirection targets ("<file").
func pathCandidates(segs []segment, argStrings []string) []string {
	var out []string
	add := func(w string) {
		w = redirection.ReplaceAllString(w, "")
		if w == "" {
			return
		}
		if strings.ContainsRune(w, '/') || strings.HasPrefix(w, ".") || strings.HasPrefix(w, "~") || strings.HasPrefix(w, "$HOME") || strings.HasPrefix(w, "${HOME}") {
			out = append(out, w)
		}
	}
	for _, s := range segs {
		for _, w := range s.Words {
			add(w)
			if _, v, ok := strings.Cut(w, "="); ok {
				add(v)
			}
		}
	}
	for _, a := range argStrings {
		add(a)
	}
	return out
}
