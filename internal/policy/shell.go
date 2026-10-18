package policy

import (
	"path/filepath"
	"strings"
)

// segment is one simple command from a shell command line, e.g. the
// `rm -rf build` in `make && rm -rf build`.
type segment struct {
	// Words are the command's words with quotes removed.
	Words []string
	// Cmd indexes the command word, after leading wrappers (sudo, env,
	// xargs, ...) and VAR=value assignments. It equals len(Words) when the
	// segment has only assignments.
	Cmd int
}

// name returns the base name of the command word ("/usr/bin/git" -> "git").
func (s segment) name() string {
	if s.Cmd >= len(s.Words) {
		return ""
	}
	return filepath.Base(s.Words[s.Cmd])
}

// args returns the words after the command word.
func (s segment) args() []string {
	if s.Cmd >= len(s.Words) {
		return nil
	}
	return s.Words[s.Cmd+1:]
}

// commandLine is the command word's base name plus its arguments, joined by
// spaces. `command:` regexes in the policy match against this string.
func (s segment) commandLine() string {
	if s.Cmd >= len(s.Words) {
		return ""
	}
	return strings.Join(append([]string{s.name()}, s.args()...), " ")
}

// maxShellDepth limits how deep `bash -c "sh -c '...'"` nesting is
// followed, so a hostile input cannot make the parser recurse forever.
const maxShellDepth = 3

// splitShell splits a command line into simple commands. It understands
// single and double quotes, backslash escapes, comments and the operators
// && || ; | & and newlines. A `sh -c "..."` (or bash, zsh, dash) has its
// script parsed as well, and those inner commands are appended.
//
// This is not a full shell parser: it does not expand variables, globs,
// $(...) or backticks. Matchers treat what it cannot resolve
// conservatively (see recursiveDeleteOutside).
func splitShell(text string) []segment {
	return splitShellDepth(text, 0)
}

func splitShellDepth(text string, depth int) []segment {
	var segs []segment
	for _, words := range shellWords(text) {
		seg := segment{Words: words, Cmd: commandIndex(words)}
		segs = append(segs, seg)
		if depth < maxShellDepth {
			if script, ok := shellScript(seg); ok {
				segs = append(segs, splitShellDepth(script, depth+1)...)
			}
		}
	}
	return segs
}

// shellWords tokenizes text into words, starting a new word list at every
// command separator.
func shellWords(text string) [][]string {
	var (
		out     [][]string
		words   []string
		cur     strings.Builder
		inWord  bool // cur holds a word, possibly empty ("")
		quote   rune // 0, '\'' or '"'
		escaped bool
	)
	endWord := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	endCommand := func() {
		endWord()
		if len(words) > 0 {
			out = append(out, words)
			words = nil
		}
	}

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case quote == '\'':
			// Inside single quotes nothing is special except the closing quote.
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case quote == '"':
			switch {
			case r == '"':
				quote = 0
			case r == '\\' && i+1 < len(runes) && strings.ContainsRune("\"\\$`\n", runes[i+1]):
				escaped = true
			default:
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped, inWord = true, true
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == '#' && !inWord:
			// A comment runs to the end of the line.
			for i+1 < len(runes) && runes[i+1] != '\n' {
				i++
			}
		case r == '&' && i > 0 && (runes[i-1] == '>' || runes[i-1] == '<'):
			// Part of a redirection such as 2>&1, not a separator.
			cur.WriteRune(r)
			inWord = true
		case r == ';' || r == '&' || r == '|' || r == '\n':
			endCommand()
		case r == ' ' || r == '\t' || r == '\r':
			endWord()
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	endCommand()
	return out
}

// wrappers run the command that follows them. Their own flags are skipped.
var wrappers = map[string]bool{
	"sudo": true, "doas": true, "env": true, "command": true, "builtin": true,
	"exec": true, "nohup": true, "time": true, "nice": true, "xargs": true,
}

// wrapperFlagsWithValue are wrapper flags whose value is the next word,
// e.g. `sudo -u postgres psql`.
var wrapperFlagsWithValue = map[string]bool{
	"-u": true, "-g": true, "-n": true, "-I": true, "-C": true, "-E": true,
}

// commandIndex finds the command word: it skips VAR=value assignments and
// wrapper commands with their flags.
func commandIndex(words []string) int {
	i := 0
	for i < len(words) {
		w := words[i]
		switch {
		case isAssignment(w):
			i++
		case wrappers[filepath.Base(w)]:
			i++
			for i < len(words) && strings.HasPrefix(words[i], "-") {
				if wrapperFlagsWithValue[words[i]] {
					i++
				}
				i++
			}
		default:
			return i
		}
	}
	return i
}

// isAssignment reports whether w is a shell variable assignment NAME=value.
func isAssignment(w string) bool {
	name, _, ok := strings.Cut(w, "=")
	if !ok || name == "" {
		return false
	}
	for i, r := range name {
		letter := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		if !letter && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}

// shellScript returns the script of `bash -c '<script>'` (also -lc, -ec).
func shellScript(s segment) (string, bool) {
	if !shells[s.name()] {
		return "", false
	}
	args := s.args()
	for i, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "c") && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}
