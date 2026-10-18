package install

import (
	"encoding/json"
	"fmt"
)

// node is one JSON value with its byte span in the source, [start, end).
// Keeping spans lets install change two values in a config file and leave
// every other byte (indentation, key order, comments) exactly as it was.
type node struct {
	kind    byte // 'o' object, 'a' array, 's' string, 'v' number/true/false/null
	start   int
	end     int
	members []member // objects, in source order
	elems   []*node  // arrays
}

type member struct {
	key   string
	value *node
}

// get returns the value of an object member, or nil.
func (n *node) get(key string) *node {
	if n == nil || n.kind != 'o' {
		return nil
	}
	// Last one wins, as in encoding/json.
	var found *node
	for _, m := range n.members {
		if m.key == key {
			found = m.value
		}
	}
	return found
}

// str decodes a string node; ok is false for any other kind.
func (n *node) str(src []byte) (string, bool) {
	if n == nil || n.kind != 's' {
		return "", false
	}
	var s string
	if err := json.Unmarshal(src[n.start:n.end], &s); err != nil {
		return "", false
	}
	return s, true
}

// scanner is a small recursive-descent JSON parser that also accepts the
// JSONC extensions VS Code uses: // and /* */ comments, trailing commas.
// It only needs to be good enough to find spans; encoding/json still
// decodes each string.
type scanner struct {
	src   []byte
	pos   int
	depth int
}

// maxDepth stops a deeply nested (hostile or broken) file from exhausting
// the stack.
const maxDepth = 512

func parseJSON(src []byte) (*node, error) {
	s := &scanner{src: src}
	n, err := s.value()
	if err != nil {
		return nil, err
	}
	s.skip()
	if s.pos != len(s.src) {
		return nil, s.errorf("unexpected data after the top-level value")
	}
	return n, nil
}

func (s *scanner) errorf(format string, args ...any) error {
	return fmt.Errorf("offset %d: %s", s.pos, fmt.Sprintf(format, args...))
}

// skip moves past whitespace and comments.
func (s *scanner) skip() {
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			s.pos++
		case c == '/' && s.pos+1 < len(s.src) && s.src[s.pos+1] == '/':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.pos++
			}
		case c == '/' && s.pos+1 < len(s.src) && s.src[s.pos+1] == '*':
			s.pos += 2
			for s.pos+1 < len(s.src) && (s.src[s.pos] != '*' || s.src[s.pos+1] != '/') {
				s.pos++
			}
			s.pos += 2
		default:
			return
		}
	}
}

func (s *scanner) value() (*node, error) {
	s.skip()
	if s.pos >= len(s.src) {
		return nil, s.errorf("unexpected end of input")
	}
	switch s.src[s.pos] {
	case '{':
		return s.object()
	case '[':
		return s.array()
	case '"':
		return s.string()
	default:
		return s.literal()
	}
}

func (s *scanner) object() (*node, error) {
	if s.depth++; s.depth > maxDepth {
		return nil, s.errorf("nested too deeply")
	}
	defer func() { s.depth-- }()
	n := &node{kind: 'o', start: s.pos}
	s.pos++ // {
	for {
		s.skip()
		if s.pos < len(s.src) && s.src[s.pos] == '}' {
			s.pos++
			n.end = s.pos
			return n, nil
		}
		k, err := s.string()
		if err != nil {
			return nil, err
		}
		key, _ := k.str(s.src)
		s.skip()
		if s.pos >= len(s.src) || s.src[s.pos] != ':' {
			return nil, s.errorf("expected ':' after key %q", key)
		}
		s.pos++
		v, err := s.value()
		if err != nil {
			return nil, err
		}
		n.members = append(n.members, member{key: key, value: v})
		if err := s.comma('}'); err != nil {
			return nil, err
		}
	}
}

func (s *scanner) array() (*node, error) {
	if s.depth++; s.depth > maxDepth {
		return nil, s.errorf("nested too deeply")
	}
	defer func() { s.depth-- }()
	n := &node{kind: 'a', start: s.pos}
	s.pos++ // [
	for {
		s.skip()
		if s.pos < len(s.src) && s.src[s.pos] == ']' {
			s.pos++
			n.end = s.pos
			return n, nil
		}
		v, err := s.value()
		if err != nil {
			return nil, err
		}
		n.elems = append(n.elems, v)
		if err := s.comma(']'); err != nil {
			return nil, err
		}
	}
}

// comma consumes a ',' or checks that the closing bracket comes next.
// A trailing comma before the bracket is accepted (JSONC).
func (s *scanner) comma(closing byte) error {
	s.skip()
	if s.pos >= len(s.src) {
		return s.errorf("unexpected end of input")
	}
	switch s.src[s.pos] {
	case ',':
		s.pos++
		return nil
	case closing:
		return nil
	}
	return s.errorf("expected ',' or %q", closing)
}

func (s *scanner) string() (*node, error) {
	if s.pos >= len(s.src) || s.src[s.pos] != '"' {
		return nil, s.errorf("expected a string")
	}
	n := &node{kind: 's', start: s.pos}
	for s.pos++; s.pos < len(s.src); s.pos++ {
		switch s.src[s.pos] {
		case '\\':
			s.pos++ // skip the escaped byte
		case '"':
			s.pos++
			n.end = s.pos
			if !json.Valid(s.src[n.start:n.end]) {
				return nil, fmt.Errorf("offset %d: invalid string", n.start)
			}
			return n, nil
		}
	}
	return nil, s.errorf("unterminated string")
}

// literal reads a number, true, false or null.
func (s *scanner) literal() (*node, error) {
	n := &node{kind: 'v', start: s.pos}
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '/' {
			break
		}
		s.pos++
	}
	n.end = s.pos
	if n.end == n.start || !json.Valid(s.src[n.start:n.end]) {
		return nil, fmt.Errorf("offset %d: invalid value", n.start)
	}
	return n, nil
}
