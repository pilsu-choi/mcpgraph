package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/pilsu-choi/mcpgraph/internal/redact"
)

// diffContext is the number of unchanged lines shown around a change.
const diffContext = 2

// writeDiff prints a unified-style diff of before → after. The hunks come
// from the edits, which are known exactly, so no general diff algorithm is
// needed (and a several-megabyte ~/.claude.json stays cheap).
//
// Config files hold secrets, so every printed line goes through
// redactJSONLine first (hard rule 2).
func writeDiff(w io.Writer, oldLabel, newLabel string, before, after []byte, edits []Edit) error {
	if len(edits) == 0 {
		return nil
	}
	oldLines, newLines := lineStarts(before), lineStarts(after)
	changes := lineChanges(before, after, oldLines, newLines, edits)

	if _, err := fmt.Fprintf(w, "--- %s\n+++ %s\n", oldLabel, newLabel); err != nil {
		return err
	}
	for _, h := range groupHunks(changes) {
		if err := writeHunk(w, h, before, after, oldLines, newLines); err != nil {
			return err
		}
	}
	return nil
}

// change is an edit expressed as inclusive line ranges in both files.
type change struct{ oldA, oldB, newA, newB int }

func lineChanges(before, after []byte, oldLines, newLines []int, edits []Edit) []change {
	sorted := append([]Edit(nil), edits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	var out []change
	delta := 0
	for _, e := range sorted {
		ns, ne := e.Start+delta, e.Start+delta+len(e.New)
		delta += len(e.New) - (e.End - e.Start)
		c := change{
			oldA: lineOf(oldLines, e.Start), oldB: lineOf(oldLines, max(e.Start, e.End-1)),
			newA: lineOf(newLines, ns), newB: lineOf(newLines, max(ns, ne-1)),
		}
		// Two edits on one line become one change.
		if n := len(out); n > 0 && c.oldA <= out[n-1].oldB {
			out[n-1].oldB = max(out[n-1].oldB, c.oldB)
			out[n-1].newB = max(out[n-1].newB, c.newB)
			continue
		}
		out = append(out, c)
	}
	return out
}

// groupHunks puts changes whose context would touch into one hunk.
func groupHunks(changes []change) [][]change {
	var out [][]change
	for _, c := range changes {
		if n := len(out); n > 0 {
			last := out[n-1][len(out[n-1])-1]
			if c.oldA-last.oldB-1 <= 2*diffContext {
				out[n-1] = append(out[n-1], c)
				continue
			}
		}
		out = append(out, []change{c})
	}
	return out
}

func writeHunk(w io.Writer, h []change, before, after []byte, oldLines, newLines []int) error {
	first, last := h[0], h[len(h)-1]
	oldStart := max(0, first.oldA-diffContext)
	oldEnd := min(len(oldLines)-1, last.oldB+diffContext)

	var body bytes.Buffer
	nOld, nNew := 0, 0
	emit := func(prefix byte, src []byte, starts []int, a, b int) {
		for i := a; i <= b; i++ {
			body.WriteByte(prefix)
			body.WriteString(redactJSONLine(line(src, starts, i)))
			body.WriteByte('\n')
			if prefix != '+' {
				nOld++
			}
			if prefix != '-' {
				nNew++
			}
		}
	}
	emit(' ', before, oldLines, oldStart, first.oldA-1)
	for i, c := range h {
		emit('-', before, oldLines, c.oldA, c.oldB)
		emit('+', after, newLines, c.newA, c.newB)
		next := oldEnd
		if i+1 < len(h) {
			next = h[i+1].oldA - 1
		}
		emit(' ', before, oldLines, c.oldB+1, next)
	}
	newStart := first.newA - (first.oldA - oldStart)
	if _, err := fmt.Fprintf(w, "@@ -%d,%d +%d,%d @@\n", oldStart+1, nOld, newStart+1, nNew); err != nil {
		return err
	}
	_, err := w.Write(body.Bytes())
	return err
}

// lineStarts returns the byte offset where each line begins.
func lineStarts(b []byte) []int {
	starts := []int{0}
	for i, c := range b {
		if c == '\n' && i+1 < len(b) {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineOf returns the index of the line holding byte offset off.
func lineOf(starts []int, off int) int {
	// sort.Search finds the first line starting after off; the line
	// before it holds off.
	return sort.Search(len(starts), func(i int) bool { return starts[i] > off }) - 1
}

// line returns line i without its line ending.
func line(b []byte, starts []int, i int) string {
	end := len(b)
	if i+1 < len(starts) {
		end = starts[i+1]
	}
	return string(bytes.TrimRight(b[starts[i]:end], "\r\n"))
}

// reJSONString matches one JSON string literal.
var reJSONString = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// redactJSONLine masks secrets inside the string literals of one line of a
// JSON config and leaves the rest of the line (indentation, punctuation)
// alone. redact.Text would treat quotes and commas as part of the words.
//
//   - `"KEY": "value"`: the value is judged with its key (redact.Value),
//     so "API_KEY": "..." is masked even when the value looks harmless.
//   - other strings on the line (array items) are judged together as an
//     argument list (redact.Args), so "--api-key", "VALUE" on one line is
//     caught too.
//
// A line that cannot be read this way (a string split over lines, or
// something that is not JSON) falls back to redact.Text.
func redactJSONLine(line string) string {
	locs := reJSONString.FindAllStringIndex(line, -1)
	if len(locs) == 0 {
		if strings.Contains(line, `"`) {
			return redact.Text(line)
		}
		return line
	}
	vals := make([]string, len(locs))
	isKey := make([]bool, len(locs))
	for i, loc := range locs {
		if json.Unmarshal([]byte(line[loc[0]:loc[1]]), &vals[i]) != nil {
			return redact.Text(line)
		}
		isKey[i] = strings.HasPrefix(strings.TrimLeft(line[loc[1]:], " \t"), ":")
	}
	out := make([]string, len(locs))
	var argIdx []int
	var args []string
	for i := range locs {
		switch {
		case isKey[i]:
			out[i] = vals[i]
		case i > 0 && isKey[i-1]:
			out[i] = redact.URL(redact.Value(vals[i-1], vals[i]))
		default:
			argIdx = append(argIdx, i)
			args = append(args, vals[i])
		}
	}
	for j, r := range redact.Args(args) {
		out[argIdx[j]] = r
	}
	var b strings.Builder
	prev := 0
	for i, loc := range locs {
		b.WriteString(line[prev:loc[0]])
		if out[i] == vals[i] {
			b.WriteString(line[loc[0]:loc[1]]) // unchanged: keep the exact bytes
		} else {
			b.Write(jsonString(out[i]))
		}
		prev = loc[1]
	}
	b.WriteString(line[prev:])
	return b.String()
}
