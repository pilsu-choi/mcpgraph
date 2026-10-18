package install

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Edit replaces src[Start:End] with New. Start == End is an insertion.
type Edit struct {
	Start, End int
	New        []byte
}

// applyEdits returns src with the edits applied. Edits must not overlap.
func applyEdits(src []byte, edits []Edit) []byte {
	sorted := append([]Edit(nil), edits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	var out bytes.Buffer
	pos := 0
	for _, e := range sorted {
		out.Write(src[pos:e.Start])
		out.Write(e.New)
		pos = e.End
	}
	out.Write(src[pos:])
	return out.Bytes()
}

// reverseEdits returns the edits that turn applyEdits(before, edits) back
// into before. Only offsets and lengths of the forward edits are needed,
// plus the original bytes, which come from the backup.
func reverseEdits(before []byte, edits []EditRecord) []Edit {
	sorted := append([]EditRecord(nil), edits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	var out []Edit
	delta := 0
	for _, e := range sorted {
		start := e.Start + delta
		out = append(out, Edit{Start: start, End: start + e.NewLen, New: before[e.Start:e.End]})
		delta += e.NewLen - (e.End - e.Start)
	}
	return out
}

// jsonString encodes s as a JSON string without HTML escaping, so "<" and
// "&" in a command line stay readable.
func jsonString(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // encoding a string cannot fail
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// jsonArray encodes strings as a one-line JSON array: ["a", "b"].
func jsonArray(items []string) []byte {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, s := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.Write(jsonString(s))
	}
	b.WriteByte(']')
	return b.Bytes()
}

// rewrite returns the edits that set an entry's command and args. When the
// entry has no "args" member, one is inserted right after "command".
func rewrite(e entry, cmd string, args []string) []Edit {
	edits := []Edit{{Start: e.command.start, End: e.command.end, New: jsonString(cmd)}}
	if e.args != nil {
		edits = append(edits, Edit{Start: e.args.start, End: e.args.end, New: jsonArray(args)})
	} else {
		ins := append([]byte(`, "args": `), jsonArray(args)...)
		edits = append(edits, Edit{Start: e.command.end, End: e.command.end, New: ins})
	}
	return edits
}
