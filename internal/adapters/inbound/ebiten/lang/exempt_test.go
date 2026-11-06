package lang

import (
	"bufio"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// exemptRule is one line of exempt.txt: `path:literal # reason`.
// path is a glob below the ebiten dir ("devtools/**" for a whole tree);
// literal is `*` (any), `/re/` (a regexp), a Go-quoted string, or the
// text itself.
type exemptRule struct {
	line int
	raw  string
	path string
	any  bool
	re   *regexp.Regexp
	text string
	hits int
}

type exemptions struct{ rules []*exemptRule }

// loadExempt reads lang/exempt.txt; every rule needs a reason.
func loadExempt(t *testing.T) *exemptions {
	t.Helper()
	f, err := os.Open("exempt.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ex, err := parseExempt(bufio.NewScanner(f))
	if err != nil {
		t.Fatal(err)
	}
	return ex
}

func parseExempt(sc *bufio.Scanner) (*exemptions, error) {
	ex := &exemptions{}
	for n := 1; sc.Scan(); n++ {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		p, rest, ok := strings.Cut(raw, ":")
		cut := strings.LastIndex(rest, " # ")
		if !ok || cut < 0 || strings.TrimSpace(rest[cut+3:]) == "" {
			return nil, lineErr(n, "want `path:literal # reason`")
		}
		r := &exemptRule{line: n, raw: raw, path: strings.TrimSpace(p)}
		lit := strings.TrimSpace(rest[:cut])
		switch {
		case lit == "*":
			r.any = true
		case len(lit) > 1 && lit[0] == '/' && lit[len(lit)-1] == '/':
			re, err := regexp.Compile(lit[1 : len(lit)-1])
			if err != nil {
				return nil, lineErr(n, err.Error())
			}
			r.re = re
		case strings.HasPrefix(lit, `"`):
			s, err := strconv.Unquote(lit)
			if err != nil {
				return nil, lineErr(n, err.Error())
			}
			r.text = s
		default:
			r.text = lit
		}
		if _, err := path.Match(r.path, ""); err != nil {
			return nil, lineErr(n, err.Error())
		}
		ex.rules = append(ex.rules, r)
	}
	return ex, sc.Err()
}

func lineErr(n int, msg string) error {
	return &exemptErr{n, msg}
}

type exemptErr struct {
	line int
	msg  string
}

func (e *exemptErr) Error() string { return "exempt.txt:" + strconv.Itoa(e.line) + ": " + e.msg }

// match reports whether a rule exempts r, counting the hit.
func (ex *exemptions) match(r rawText) bool {
	for _, x := range ex.rules {
		if !x.pathMatch(r.file) {
			continue
		}
		if x.any || (x.re != nil && x.re.MatchString(r.text)) || (x.re == nil && x.text == r.text) {
			x.hits++
			return true
		}
	}
	return false
}

func (x *exemptRule) pathMatch(file string) bool {
	if dir, ok := strings.CutSuffix(x.path, "/**"); ok {
		return strings.HasPrefix(file, dir+"/")
	}
	ok, _ := path.Match(x.path, file)
	return ok
}

// unused lists rules that exempted nothing (stale entries).
func (ex *exemptions) unused() []*exemptRule {
	var out []*exemptRule
	for _, x := range ex.rules {
		if x.hits == 0 {
			out = append(out, x)
		}
	}
	return out
}

func TestExemptRules(t *testing.T) {
	src := "# comment\n" +
		"devtools/**:* # developer tools\n" +
		"hud/*.go:/^[A-Z]$/ # key letters\n" +
		"hud/global.go:\"  pad\" # quoted\n" +
		"hud/global.go:Exact text # an id\n"
	ex, err := parseExempt(bufio.NewScanner(strings.NewReader(src)))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		file, text string
		want       bool
	}{
		{"devtools/panel.go", "Anything", true},
		{"devtools/sub/x.go", "Anything", true},
		{"devtoolsx/a.go", "Anything", false},
		{"hud/bar.go", "Q", true},
		{"hud/bar.go", "QQ", false},
		{"hud/global.go", "  pad", true},
		{"hud/global.go", "Exact text", true},
		{"hud/other.go", "Exact text", false},
	}
	for _, c := range cases {
		if got := ex.match(rawText{file: c.file, text: c.text}); got != c.want {
			t.Errorf("%s %q: %v, want %v", c.file, c.text, got, c.want)
		}
	}
	if len(ex.unused()) != 0 {
		t.Errorf("unused rules: %d", len(ex.unused()))
	}
	for _, bad := range []string{"hud/a.go:Text\n", "hud/a.go:Text # \n", "nocolon # x\n", "hud/a.go:/[/ # bad re\n"} {
		if _, err := parseExempt(bufio.NewScanner(strings.NewReader(bad))); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
