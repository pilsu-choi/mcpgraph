// Command termsvg renders plain terminal output as an SVG "screenshot" of a
// terminal window, so README captures can be regenerated from real runs
// instead of being hand-made images.
//
// Usage:
//
//	some-command | go run ./tools/termsvg -title "mcpgraph scan" -o docs/images/scan.svg
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Layout constants, in pixels. charWidth is generous for common monospace
// fonts at fontSize so long lines are not clipped.
const (
	fontSize   = 13
	charWidth  = 7.9
	lineHeight = 18
	padX       = 16
	padTop     = 44 // room for the window title bar
	padBottom  = 16
	maxCols    = 170
)

// Colors (GitHub dark palette).
const (
	colBG      = "#0d1117"
	colBar     = "#161b22"
	colText    = "#c9d1d9"
	colDim     = "#8b949e"
	colPrompt  = "#7ee787"
	colHeader  = "#79c0ff"
	colHigh    = "#ff7b72"
	colMedium  = "#d29922"
	colMask    = "#d2a8ff"
	colOK      = "#3fb950"
	colInfoKey = "#a5d6ff"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "termsvg:", err)
		os.Exit(1)
	}
}

// run holds the real work so its deferred Close runs before main calls
// os.Exit (os.Exit skips deferred calls).
func run() error {
	title := flag.String("title", "terminal", "window title")
	out := flag.String("o", "", "output file (default stdout)")
	flag.Parse()

	lines, err := readLines(os.Stdin)
	if err != nil {
		return err
	}
	w := io.Writer(os.Stdout)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	_, err = io.WriteString(w, Render(*title, lines))
	return err
}

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.ReplaceAll(sc.Text(), "\t", "    "))
	}
	return lines, sc.Err()
}

// Render returns the SVG document for the given lines.
func Render(title string, lines []string) string {
	cols := 40
	for i, l := range lines {
		r := []rune(l)
		if len(r) > maxCols {
			lines[i] = string(r[:maxCols-1]) + "…"
			r = r[:maxCols]
		}
		if len(r) > cols {
			cols = len(r)
		}
	}
	width := int(float64(cols)*charWidth) + 2*padX
	height := padTop + len(lines)*lineHeight + padBottom

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n", width, height, width, height)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="8" fill="%s"/>`+"\n", width, height, colBG)
	fmt.Fprintf(&b, `<rect width="%d" height="30" rx="8" fill="%s"/><rect y="22" width="%d" height="8" fill="%s"/>`+"\n", width, colBar, width, colBar)
	for i, c := range []string{"#ff5f56", "#ffbd2e", "#27c93f"} {
		fmt.Fprintf(&b, `<circle cx="%d" cy="15" r="6" fill="%s"/>`+"\n", 18+i*20, c)
	}
	fmt.Fprintf(&b, `<text x="%d" y="19" fill="%s" font-family="-apple-system,Segoe UI,Helvetica,Arial,sans-serif" font-size="12" text-anchor="middle">%s</text>`+"\n",
		width/2, colDim, escape(title))
	fmt.Fprintf(&b, `<g font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,'Liberation Mono',monospace" font-size="%d" fill="%s" xml:space="preserve" style="white-space:pre">`+"\n", fontSize, colText)

	section := ""
	for i, l := range lines {
		section = nextSection(section, l)
		y := padTop + i*lineHeight + fontSize
		fmt.Fprintf(&b, `<text x="%d" y="%d">%s</text>`+"\n", padX, y, colorLine(l, section))
	}
	b.WriteString("</g>\n</svg>\n")
	return b.String()
}

// nextSection tracks whether we are inside the HIGH or MEDIUM block of a
// findings report, so rule IDs can be colored by severity.
func nextSection(cur, line string) string {
	switch {
	case strings.HasPrefix(line, "HIGH ("):
		return "high"
	case strings.HasPrefix(line, "MEDIUM ("):
		return "medium"
	case strings.HasPrefix(line, "$ ") || strings.HasPrefix(line, "FINDINGS:"):
		return ""
	}
	return cur
}

var inline = regexp.MustCompile(`\*\*\*\*[^\s",&]{0,4}|\[R\d\]|"level":"(INFO|WARN|ERROR|DEBUG)"|"msg":"[^"]*"|\bok\b|--- PASS|--- FAIL|\bPASS\b|\bFAIL\b`)

// colorLine escapes a line and wraps highlighted parts in <tspan>s.
func colorLine(line, section string) string {
	switch {
	case strings.HasPrefix(line, "$ "):
		return span(colPrompt, "$ ") + span("#ffffff", line[2:])
	case strings.HasPrefix(line, "HIGH ("):
		return bold(colHigh, line)
	case strings.HasPrefix(line, "MEDIUM ("):
		return bold(colMedium, line)
	case isHeader(line):
		return bold(colHeader, line)
	case strings.HasPrefix(line, "#"):
		return span(colDim, line)
	}

	var b strings.Builder
	last := 0
	for _, m := range inline.FindAllStringIndex(line, -1) {
		b.WriteString(escape(line[last:m[0]]))
		b.WriteString(highlight(line[m[0]:m[1]], section))
		last = m[1]
	}
	b.WriteString(escape(line[last:]))
	return b.String()
}

func highlight(tok, section string) string {
	switch {
	case strings.HasPrefix(tok, "****"):
		return bold(colMask, tok)
	case strings.HasPrefix(tok, "[R"):
		if section == "high" {
			return bold(colHigh, tok)
		}
		return bold(colMedium, tok)
	case strings.Contains(tok, `"level":"WARN"`), strings.Contains(tok, `"level":"ERROR"`), strings.Contains(tok, "FAIL"):
		return bold(colHigh, tok)
	case strings.HasPrefix(tok, `"level"`):
		return span(colDim, tok)
	case strings.HasPrefix(tok, `"msg"`):
		return span(colInfoKey, tok)
	default: // ok, PASS
		return bold(colOK, tok)
	}
}

// isHeader matches all-caps section titles such as "MCP SERVERS (10)" and
// table header rows such as "APP  SERVER  TRANSPORT".
func isHeader(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "[") {
		return false
	}
	letters := 0
	for _, r := range t {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			letters++
		}
	}
	return letters >= 3 && !strings.ContainsAny(t, "/@:")
}

func span(color, s string) string {
	return fmt.Sprintf(`<tspan fill="%s">%s</tspan>`, color, escape(s))
}

func bold(color, s string) string {
	return fmt.Sprintf(`<tspan fill="%s" font-weight="bold">%s</tspan>`, color, escape(s))
}

// escape makes s safe as SVG text. Spaces become non-breaking spaces:
// some renderers collapse runs of normal spaces even with xml:space, which
// would break column alignment.
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", " ", "&#160;").Replace(s)
}
