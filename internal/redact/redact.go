// Package redact detects secret-looking values and masks them.
//
// Every string that leaves a collector (args, URLs, command lines) passes
// through this package first, so the graph, the snapshots and every output
// format only ever see masked values.
package redact

import (
	"math"
	"net/url"
	"strings"
)

// maskPrefix replaces the hidden part of a secret.
const maskPrefix = "****"

// minRevealLen is the shortest value for which we reveal the last 4
// characters. Shorter values are fully masked so that we never reveal a
// large fraction of a short secret.
const minRevealLen = 12

// Mask hides a secret value: "****" plus at most its last 4 characters.
func Mask(v string) string {
	if len(v) < minRevealLen {
		return maskPrefix
	}
	return maskPrefix + v[len(v)-4:]
}

// knownPrefixes are token prefixes used by common providers. A value that
// starts with one of these is treated as a secret regardless of entropy.
var knownPrefixes = []string{
	"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_", // GitHub
	"glpat-",                     // GitLab
	"sk-ant-", "sk-proj-", "sk-", // Anthropic, OpenAI
	"xoxb-", "xoxp-", "xoxa-", "xapp-", // Slack
	"AKIA", "ASIA", // AWS access key IDs
	"AIza",                             // Google API keys
	"sk_live_", "rk_live_", "sk_test_", // Stripe
	"npm_",     // npm
	"hf_",      // Hugging Face
	"ntn_",     // Notion
	"lin_api_", // Linear
	"pplx-",    // Perplexity
	"eyJ",      // JWT (base64 of `{"`)
}

// secretKeyWords mark a key (env var, flag, header, query parameter) whose
// value is a secret no matter what the value looks like.
var secretKeyWords = []string{
	"TOKEN", "SECRET", "PASSWORD", "PASSWD", "API_KEY", "APIKEY",
	"ACCESS_KEY", "PRIVATE_KEY", "CREDENTIAL", "AUTH", "SESSION", "COOKIE",
}

// IsSecretKey reports whether a key name suggests its value is a secret,
// e.g. GITHUB_TOKEN, --api-key, Authorization.
func IsSecretKey(key string) bool {
	k := strings.ToUpper(strings.TrimLeft(key, "-"))
	k = strings.ReplaceAll(k, "-", "_")
	for _, w := range secretKeyWords {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}

// IsReference reports whether v points at a secret stored elsewhere instead
// of containing it, e.g. "${GITHUB_TOKEN}", "${env:TOKEN}" or
// "${input:token}" (VS Code). Such values are safe to keep.
func IsReference(v string) bool {
	v = strings.TrimSpace(v)
	if strings.Contains(v, "${") {
		return true
	}
	// A bare "$NAME" is a shell-style reference.
	if len(v) > 1 && v[0] == '$' && isEnvName(v[1:]) {
		return true
	}
	return false
}

func isEnvName(s string) bool {
	for _, r := range s {
		ok := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return s != ""
}

// Detect reports whether value looks like a secret and why.
//
// key may be empty (for a positional argument). When key is secret-named
// (e.g. API_KEY) any non-empty literal value counts as a secret.
func Detect(key, value string) (bool, string) {
	v := strings.TrimSpace(value)
	if v == "" || IsReference(v) {
		return false, ""
	}
	// "Bearer abc..." / "Basic abc..." headers: inspect the token part.
	if fields := strings.Fields(v); len(fields) == 2 && isAuthScheme(fields[0]) {
		return true, "authorization header value"
	}
	for _, p := range knownPrefixes {
		if strings.HasPrefix(v, p) && len(v) >= len(p)+8 {
			return true, "known token prefix " + p
		}
	}
	if key != "" && IsSecretKey(key) && !looksLikePlainValue(v) {
		return true, "secret-named key " + key
	}
	if looksHighEntropy(v) {
		return true, "high-entropy string"
	}
	return false, ""
}

func isAuthScheme(s string) bool {
	switch strings.ToLower(s) {
	case "bearer", "basic", "token":
		return true
	}
	return false
}

// looksLikePlainValue filters out values that sit under a secret-named key
// but are clearly not secrets, such as booleans or file paths
// (GOOGLE_APPLICATION_CREDENTIALS=/path/key.json is a file reference, which
// derive code records as a "file" credential instead).
func looksLikePlainValue(v string) bool {
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "0", "1", "none", "null":
		return true
	}
	return LooksLikePath(v)
}

// LooksLikePath reports whether v is a filesystem path.
func LooksLikePath(v string) bool {
	return strings.HasPrefix(v, "/") || strings.HasPrefix(v, "~/") ||
		strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../") || v == "~"
}

// looksHighEntropy flags long random-looking strings, which is how most
// API keys without a known prefix look.
func looksHighEntropy(v string) bool {
	// Spaces, paths, versions ("pkg==1.2.3", "pkg@1.2") and host:port
	// strings are not tokens.
	if len(v) < 20 || strings.ContainsAny(v, " /\\.@:") {
		return false
	}
	if isUUID(v) {
		return false
	}
	var lower, upper, digit bool
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		}
	}
	// Require digits plus at least one letter case: package names such as
	// "@modelcontextprotocol/server-everything" or long words do not have
	// this mix.
	if !digit || (!lower && !upper) {
		return false
	}
	return shannonEntropy(v) >= 3.5
}

// isUUID reports whether v has the 8-4-4-4-12 hex layout of a UUID. UUIDs
// are identifiers (project IDs, workspace IDs), not secrets.
func isUUID(v string) bool {
	parts := strings.Split(v, "-")
	if len(parts) != 5 {
		return false
	}
	want := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != want[i] || strings.Trim(p, "0123456789abcdefABCDEF") != "" {
			return false
		}
	}
	return true
}

// shannonEntropy returns the average bits of information per character.
// Random base64/hex tokens score ~4-6; English words score ~2.5-3.5.
func shannonEntropy(s string) float64 {
	counts := map[rune]int{}
	total := 0
	for _, r := range s {
		counts[r]++
		total++
	}
	var h float64
	for _, c := range counts {
		p := float64(c) / float64(total)
		h -= p * math.Log2(p)
	}
	return h
}

// Value masks v if it looks like a secret under the given key, and returns
// it unchanged otherwise.
func Value(key, v string) string {
	if ok, _ := Detect(key, v); ok {
		return Mask(v)
	}
	return v
}

// Args redacts a command's argument list. It masks:
//   - secret-looking positional values,
//   - the value in "--flag=value" when the flag or value is secret-like,
//   - the argument after a secret-named flag ("--api-key VALUE"),
//   - passwords and secret query parameters inside URL arguments.
func Args(args []string) []string {
	out := make([]string, len(args))
	prevSecretFlag := false
	for i, a := range args {
		switch {
		case prevSecretFlag && !strings.HasPrefix(a, "-"):
			out[i] = maskUnlessSafe(a)
		case strings.HasPrefix(a, "-") && strings.Contains(a, "="):
			k, v, _ := strings.Cut(a, "=")
			if strings.Contains(v, "://") {
				out[i] = k + "=" + URL(v)
			} else {
				out[i] = k + "=" + Value(k, v)
			}
		case strings.Contains(a, "://"):
			out[i] = URL(a)
		default:
			out[i] = Value("", a)
		}
		prevSecretFlag = strings.HasPrefix(a, "-") && !strings.Contains(a, "=") && IsSecretKey(a)
	}
	return out
}

func maskUnlessSafe(v string) string {
	if IsReference(v) || looksLikePlainValue(v) {
		return v
	}
	return Mask(v)
}

// URL masks the password part of the user info and secret-looking query
// parameter values. A value that does not parse as a URL is passed to
// Value instead.
func URL(raw string) string {
	if !strings.Contains(raw, "://") {
		return Value("", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Do not echo something we could not understand.
		return Mask(raw)
	}
	if u.User != nil {
		if _, hasPass := u.User.Password(); hasPass {
			u.User = url.UserPassword(u.User.Username(), maskPrefix)
		} else if ok, _ := Detect("", u.User.Username()); ok {
			u.User = url.User(maskPrefix)
		}
	}
	q := u.Query()
	changed := false
	for k, vals := range q {
		for i, v := range vals {
			if ok, _ := Detect(k, v); ok {
				vals[i] = Mask(v)
				changed = true
			}
		}
	}
	if changed {
		u.RawQuery = q.Encode()
	}
	// url.URL.String escapes '*' in user info; undo that so the mask reads
	// the same everywhere.
	return strings.ReplaceAll(u.String(), "%2A", "*")
}

// Cmdline redacts a whole command line given as an argument slice and
// returns both the redacted slice and the joined string.
func Cmdline(args []string) ([]string, string) {
	r := Args(args)
	return r, strings.Join(r, " ")
}

// Text redacts free-form text such as a shell command or a SQL query, for
// places that show what an agent tried to run (decision summaries, the
// audit log). Whitespace is collapsed to single spaces. On top of Args it
// masks:
//   - environment assignments "NAME=value" (PGPASSWORD=..., API_KEY=...),
//   - the word after "Bearer", "Basic", "token" or "password" (headers,
//     ALTER USER ... PASSWORD '...').
func Text(s string) string {
	fields := strings.Fields(s)
	// Quotes around a word ("--token=abc", 'Bearer') would hide it from
	// detection, so detect on the unquoted core and keep the original
	// word when nothing needed masking.
	cores := make([]string, len(fields))
	for i, f := range fields {
		cores[i] = strings.Trim(f, `"'`)
	}
	masked := Args(cores)
	out := make([]string, len(fields))
	for i, f := range fields {
		core := cores[i]
		switch {
		case i > 0 && maskNextWord(cores[i-1]) && core != "":
			out[i] = Mask(core)
		case isEnvAssignment(core):
			name, v, _ := strings.Cut(core, "=")
			if strings.Contains(v, "://") {
				out[i] = name + "=" + URL(v)
			} else {
				out[i] = name + "=" + Value(name, v)
			}
		case masked[i] != core:
			out[i] = masked[i]
		default:
			out[i] = f
		}
	}
	return strings.Join(out, " ")
}

// maskNextWord reports whether the word after w is a credential.
func maskNextWord(w string) bool {
	switch strings.ToLower(strings.TrimSuffix(w, ":")) {
	case "bearer", "basic", "token", "password":
		return true
	}
	return false
}

// isEnvAssignment reports whether w looks like NAME=value with a shell
// variable name, e.g. PGPASSWORD=secret.
func isEnvAssignment(w string) bool {
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
