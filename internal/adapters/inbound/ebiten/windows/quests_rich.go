package windows

import (
	"image/color"
	"strconv"
	"strings"
	"unicode"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	quest "rimworld-go/internal/application/game/quest"
)

// Coloured spans in the detail body (QST-127). A detail line comes from
// the game as a key with typed arguments, so its runs come from the
// arguments: the line's words are formatted with a marker in each
// argument's place, and the text between markers is plain while each
// argument is a run in its kind's role. Nothing reads the rendered text.

// runRole is a run's colour role.
type runRole uint8

const (
	runPlain   runRole = iota
	runGiver           // the quest giver's name: warm
	runFaction         // another faction: its relation colour (red when hostile)
	runNumber          // numbers, counts and times: highlighted
)

// questRun is a piece of a detail line in one role; ref is the faction of
// a runFaction or runGiver run.
type questRun struct {
	s    string
	role runRole
	ref  int
}

// argMark is argument i's place holder in a formatted line: control
// characters no word or name holds.
func argMark(i int) string { return "\x1e" + strconv.Itoa(i) + "\x1f" }

// argRole is the role argument a shows in, for a quest given by giver.
func argRole(a quest.Arg, giver int) runRole {
	switch a.Kind {
	case quest.ArgFaction:
		if int(a.N) == giver && giver != 0 {
			return runGiver
		}
		return runFaction
	case quest.ArgNum, quest.ArgTick:
		return runNumber
	}
	return runPlain
}

// questRuns is detail line l as runs. An unknown key shows the key and
// its arguments, each argument still in its role.
func questRuns(tr questTexts, l quest.Line, giver int, now uint64) []questRun {
	args := make([]questRun, len(l.Args))
	marks := make([]any, len(l.Args))
	for i, a := range l.Args {
		args[i] = questRun{s: questArg(tr, a, now), role: argRole(a, giver), ref: int(a.N)}
		marks[i] = argMark(i)
	}
	tmpl, ok := questLineKnown(tr, l.Key, marks)
	if !ok {
		out := []questRun{{s: l.Key}}
		for _, a := range args {
			out = append(out, questRun{s: " "}, a)
		}
		return out
	}
	var out []questRun
	for tmpl != "" {
		i := strings.IndexByte(tmpl, '\x1e')
		j := strings.IndexByte(tmpl, '\x1f')
		if i < 0 || j < i {
			out = append(out, questRun{s: tmpl})
			break
		}
		if i > 0 {
			out = append(out, questRun{s: tmpl[:i]})
		}
		if n, err := strconv.Atoi(tmpl[i+1 : j]); err == nil && n < len(args) {
			out = append(out, args[n])
		}
		tmpl = tmpl[j+1:]
	}
	return out
}

// runsText is the runs' text, uncoloured.
func runsText(rs []questRun) string {
	var b strings.Builder
	for _, r := range rs {
		b.WriteString(r.s)
	}
	return b.String()
}

// runSpan is a laid-out piece of runs: its text, x and line, and the run
// it came from.
type runSpan struct {
	s    string
	x    float32
	line int
	run  int
}

// runWord is a word with no space inside: one or more pieces of runs
// ("Quorhollow" then "." from the next run).
type runWord struct {
	segs []runSpan // x is the offset in the word
	w    float32
}

// runWords splits runs into words at spaces; a piece's run index is kept.
func runWords(rs []questRun, st gfx.TextStyle) []runWord {
	var out []runWord
	var cur runWord
	flush := func() {
		if len(cur.segs) > 0 {
			out = append(out, cur)
		}
		cur = runWord{}
	}
	for ri, r := range rs {
		start := -1
		piece := func(end int) {
			if start >= 0 {
				s := r.s[start:end]
				cur.segs = append(cur.segs, runSpan{s: s, x: cur.w, run: ri})
				cur.w += gfx.TextW(s, st)
				start = -1
			}
		}
		for i, c := range r.s {
			if unicode.IsSpace(c) {
				piece(i)
				flush()
				continue
			}
			if start < 0 {
				start = i
			}
		}
		piece(len(r.s))
	}
	flush()
	return out
}

// layoutRuns wraps runs inside maxW at spaces, as gfx.WrapPx does a
// string: a word wider than the line is cut by runes. It returns the spans
// and the number of lines (at least 1).
func layoutRuns(rs []questRun, st gfx.TextStyle, maxW float32) ([]runSpan, int) {
	space := gfx.TextW(" ", st)
	var out []runSpan
	x, line := float32(0), 0
	for _, w := range runWords(rs, st) {
		if x > 0 && x+space+w.w > maxW {
			x, line = 0, line+1
		}
		if x > 0 {
			x += space
		}
		if w.w <= maxW-x {
			for _, sg := range w.segs {
				out = append(out, runSpan{s: sg.s, x: x + sg.x, line: line, run: sg.run})
			}
			x += w.w
			continue
		}
		for _, sg := range w.segs { // too wide for any line: cut it by runes
			for _, part := range cutRunes(sg.s, st, maxW-x, maxW) {
				if part == "" {
					x, line = 0, line+1
					continue
				}
				out = append(out, runSpan{s: part, x: x, line: line, run: sg.run})
				x += gfx.TextW(part, st)
			}
		}
	}
	return out, line + 1
}

// cutRunes cuts s into parts: the first fits in first, the others in
// full; an empty part marks a line break before the next part.
func cutRunes(s string, st gfx.TextStyle, first, full float32) []string {
	var out []string
	room := first
	for s != "" {
		r := []rune(s)
		n := len(r)
		for n > 0 && gfx.TextW(string(r[:n]), st) > room {
			n--
		}
		if n == 0 && room < full { // nothing fits the rest of this line
			out, room = append(out, ""), full
			continue
		}
		n = max(n, 1)
		out = append(out, string(r[:n]))
		if s = string(r[n:]); s != "" {
			out, room = append(out, ""), full
		}
	}
	return out
}

// runColor is role's colour; facs maps a faction name to its relation
// colour (factions without a row fall back to the plain text colour).
func runColor(r questRun, name func(ref int) string, facs []Faction) color.RGBA {
	switch r.role {
	case runGiver:
		return uikit.Accent
	case runNumber:
		return uikit.Heading
	case runFaction:
		n := name(r.ref)
		for _, f := range facs {
			if f.Name == n && n != "" {
				return f.Color
			}
		}
	}
	return uikit.Text
}
