// Command termsvg renders plain terminal output as an SVG "screenshot" of a
// terminal window, so README captures can be regenerated from real runs
// instead of being hand-made images.
//
// Usage:
//
//	some-command | go run ./tools/termsvg -title "mcpgraph scan" -o docs/images/scan.svg
//
// With -animate the input is a timed transcript and the output is an
// animated SVG (CSS keyframes only, so it plays inside a README <img>):
//
//	go run ./tools/termsvg -animate -o docs/images/demo.svg < transcript.txt
//
// Transcript lines are plain terminal lines. A line "#@ 1.5" adds a pause of
// 1.5 seconds before the next line; lines starting with "$ " are typed out.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
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
	animate := flag.Bool("animate", false, "input is a timed transcript; write an animated SVG")
	flag.Parse()

	lines, err := readLines(os.Stdin)
	if err != nil {
		return err
	}
	render := func() string { return Render(*title, lines) }
	if *animate {
		render = func() string { return RenderAnimated(*title, lines) }
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
	_, err = io.WriteString(w, render())
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
	return document(title, lines, "", func(int) string { return "" })
}

// document is the part static and animated output share: the window, the
// title bar and one <text> per line. css goes into a <style> element and
// attrs(i) adds attributes to line i's <text>.
func document(title string, lines []string, css string, attrs func(i int) string) string {
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
	if css != "" {
		fmt.Fprintf(&b, "<style>%s</style>\n", css)
	}
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
		fmt.Fprintf(&b, `<text x="%d" y="%d"%s>%s</text>`+"\n", padX, y, attrs(i), colorLine(l, section))
	}
	b.WriteString("</g>\n</svg>\n")
	return b.String()
}

// Timing constants of the animated mode, in seconds.
const (
	lineGap  = 0.12 // between two output lines
	typeRate = 0.04 // per typed character
	holdEnd  = 4.0  // everything stays visible this long before the loop restarts
)

// RenderAnimated renders a timed transcript (see the package comment) as an
// SVG whose lines appear one after another and loop forever. Each line gets
// its own @keyframes: output lines switch from hidden to visible; "$ "
// command lines are revealed by a clip-path that grows in one step per
// character. There are no scripts, so GitHub shows it in an <img>.
func RenderAnimated(title string, raw []string) string {
	var lines []string
	var start, dur []float64
	t, pause := 0.0, 0.0
	for _, l := range raw {
		if rest, ok := strings.CutPrefix(l, "#@ "); ok {
			if sec, err := strconv.ParseFloat(strings.TrimSpace(rest), 64); err == nil {
				pause += sec
			}
			continue
		}
		t += lineGap + pause
		pause = 0
		d := 0.0
		if strings.HasPrefix(l, "$ ") {
			d = float64(len([]rune(l))) * typeRate
		}
		lines, start, dur = append(lines, l), append(start, t), append(dur, d)
		t += d
	}
	total := t + pause + holdEnd

	var css strings.Builder
	fmt.Fprintf(&css, "g text{animation-duration:%.2fs;animation-iteration-count:infinite;animation-timing-function:linear}", total)
	for i, l := range lines {
		p0, p1 := start[i]/total*100, (start[i]+dur[i])/total*100
		if dur[i] == 0 {
			// Hidden, then visible from p0 until just before the loop.
			fmt.Fprintf(&css, "@keyframes k%d{0%%,%.3f%%{opacity:0}%.3f%%,99.9%%{opacity:1}100%%{opacity:0}}", i, p0, p0+0.001)
			continue
		}
		steps := len([]rune(l))
		fmt.Fprintf(&css, "@keyframes k%d{0%%,%.3f%%{clip-path:inset(0 100%% 0 0);animation-timing-function:steps(%d,end)}%.3f%%,99.9%%{clip-path:inset(0)}100%%{clip-path:inset(0 100%% 0 0)}}",
			i, p0, steps, p1)
	}
	return document(title, lines, css.String(), func(i int) string {
		return fmt.Sprintf(` style="animation-name:k%d"`, i)
	})
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
