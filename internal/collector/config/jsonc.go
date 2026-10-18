package config

// StripJSONC removes // and /* */ comments and trailing commas so that a
// JSONC document can be decoded by encoding/json. It tracks whether it is
// inside a string so that "http://..." is left alone.
func StripJSONC(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(in) {
				i++
				out = append(out, in[i])
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			if i < len(in) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && (in[i] != '*' || in[i+1] != '/') {
				i++
			}
			i++ // skip the closing '/'
		case c == ',' && nextNonSpaceCloses(in, i+1):
			// drop trailing comma
		default:
			out = append(out, c)
		}
	}
	return out
}

// nextNonSpaceCloses reports whether the next byte from i that is not
// whitespace or part of a comment is '}' or ']'.
func nextNonSpaceCloses(in []byte, i int) bool {
	for i < len(in) {
		switch {
		case in[i] == ' ' || in[i] == '\t' || in[i] == '\n' || in[i] == '\r':
			i++
		case in[i] == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
		case in[i] == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && (in[i] != '*' || in[i+1] != '/') {
				i++
			}
			i += 2
		default:
			return in[i] == '}' || in[i] == ']'
		}
	}
	return false
}
