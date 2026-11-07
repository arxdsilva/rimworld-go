package game

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/pkg/msgfmt"
)

// OPT-001 Wave 4: every message and letter the simulation posts carries
// its text as a catalog key and args (Msg), and Msg prints the posted
// English byte for byte, so the client can translate it and English
// players see no change.

// msgCheck tallies what checkPosts saw.
type msgCheck struct {
	keyed, plain map[string]bool // keyed: formats with args; plain: finished text
	letters      int
}

// checkPosts checks every message and letter g holds right now.
func (c *msgCheck) checkPosts(t *testing.T, g *Game) {
	t.Helper()
	for _, m := range g.messages {
		c.check(t, "message", m.Text, m.Msg)
	}
	for _, l := range g.letterViews() {
		c.letters++
		c.check(t, "letter "+l.Title, l.Text, l.Msg)
	}
}

func (c *msgCheck) check(t *testing.T, what, text string, m msgfmt.Text) {
	t.Helper()
	switch {
	case text == "":
		return
	case m.Format == "":
		t.Errorf("%s %q has no Msg", what, text)
	case m.String() != text:
		t.Errorf("%s: Msg %q %q prints %q, posted %q", what, m.Format, m.Args(), m.String(), text)
	case strings.Contains(text, "%!"):
		t.Errorf("%s %q: a verb and its argument disagree", what, text)
	case m.Packed != "" && m.Format != "%s":
		c.keyed[m.Format] = true
	default:
		c.plain[m.Format] = true
	}
}

// TestPostsCarryTheirFormat runs a colony through every incident that
// fits, a raid and a few days, and checks every message and letter on the
// way: none lacks Msg, and each Msg prints its Text.
func TestPostsCarryTheirFormat(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 11, Width: 64, Height: 64})
	c := &msgCheck{keyed: map[string]bool{}, plain: map[string]bool{}}
	g.Step(500)
	c.checkPosts(t, g)
	fired := 0
	for _, d := range incident.All() {
		if g.FireIncident(d.Key) {
			fired++
		}
		c.checkPosts(t, g)
		g.Step(20)
		c.checkPosts(t, g)
	}
	g.Raid(3)
	c.checkPosts(t, g)
	for range 60 {
		g.Step(50)
		c.checkPosts(t, g)
	}
	// the hand-posted kinds, a % in finished text among them
	g.letter(letters.Info, "Plain", "Stocks at 100% of last year.", 0)
	letterf(g, letters.Good, "Keyed", 0, "%s found %d silver (%d%%).", "Ada", 40, 5)
	pawnLetterf(g, letters.Caution, "Keyed about", 0, "%s is %s.", "Bo", "sad")
	g.worldNote("A note at %d%%.", 7)
	g.notice(1, 1, 0, "%d deer and %s.", 3, "a hare")
	c.checkPosts(t, g)
	if fired < 5 || c.letters == 0 || len(c.keyed) < 8 {
		t.Fatalf("too little posted to check: %d incidents fired, %d letters, %d keyed formats", fired, c.letters, len(c.keyed))
	}
	t.Logf("%d incidents fired; %d keyed formats, %d finished texts", fired, len(c.keyed), len(c.plain))
}

// TestLettersKeepMsgThroughSave: a saved letter keeps its key and args.
func TestLettersKeepMsgThroughSave(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 3, Width: 40, Height: 40})
	letterf(g, letters.Good, "Keyed", 0, "%s found %d silver.", "Ada", 40)
	g.letter(letters.Info, "Plain", "Half of 50% is 25%.", 0)
	var buf bytes.Buffer
	if err := g.Save(&buf, "msg"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	want := g.letterViews()
	got := h.letterViews()
	if len(got) != len(want) || len(got) < 2 {
		t.Fatalf("letters after load: %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i].Msg != want[i].Msg || got[i].Msg.String() != got[i].Text {
			t.Errorf("letter %q: Msg %+v after load, want %+v printing %q", got[i].Title, got[i].Msg, want[i].Msg, got[i].Text)
		}
	}
	if m := got[len(got)-2].Msg; m.Format != "%s found %s silver." || len(m.Args()) != 2 {
		t.Errorf("keyed letter saved as %+v", m)
	}
}

// postFormats is where each posting call takes its format: the root's
// constructors (by name) and, per sub-package, the host methods that post.
var postFormats = map[string]map[string]int{
	".":        {"message": 2, "threat": 3, "worldNote": 0, "notice": 3, "letterf": 4, "pawnLetterf": 4},
	"maps":     {"Letter": 3, "Note": 0, "DemandLetter": 1},
	"travel":   {"Letter": 3, "Message": 2},
	"industry": {"Letter": 3},
	"ship":     {"Letter": 4, "Message": 2},
	"visit":    {"Letter": 3, "Message": 2},
	"gather":   {"Note": 3, "GoodLetter": 4},
	"raid":     {"Threat": 3, "Message": 2},
	"herd":     {"Message": 2},
	"aqua":     {"Message": 2},
	"combat":   {"Message": 2},
	"craft":    {"Message": 2},
	"care":     {"Message": 2},
	"orders":   {"Message": 2},
	"prison":   {"Message": 2},
}

// formatExceptions are posting calls whose format is not a literal the
// client's extractor can read, with why ("file:expression").
var formatExceptions = map[string]string{
	"heat.go:how": "picks one of two literal formats (froze or heatstroke) just above the call",
}

// TestPostFormatsAreLiterals: every posting call in the game tree passes
// a format the client's extractor can read (a string literal, literals
// joined with +, a constant of one, msgfmt.Plain of finished text, or a
// wrapper forwarding its own format parameter), and passes as many args
// as the format has verbs (vet no longer checks these calls: msgfmt.Of
// formats them, not fmt.Sprintf).
func TestPostFormatsAreLiterals(t *testing.T) {
	t.Parallel()
	calls := 0
	for dir, funcs := range postFormats {
		fset := token.NewFileSet()
		files, consts := parseDir(t, fset, dir)
		for _, f := range files {
			name := filepath.Base(fset.File(f.Pos()).Name())
			ast.Inspect(f, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}
				params := map[string]bool{}
				for _, p := range fn.Type.Params.List {
					for _, id := range p.Names {
						params[id.Name] = true
					}
				}
				ast.Inspect(fn, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					var fname string
					switch fun := call.Fun.(type) {
					case *ast.SelectorExpr:
						fname = fun.Sel.Name
					case *ast.Ident:
						fname = fun.Name
					}
					at, ok := funcs[fname]
					if !ok || len(call.Args) <= at || dir != "." && !isSelector(call.Fun) {
						return true
					}
					calls++
					checkFormatArg(t, fset, name, call, at, consts, params)
					return true
				})
				return false
			})
		}
	}
	if calls < 250 {
		t.Fatalf("only %d posting calls found: the scan lost its way", calls)
	}
}

func isSelector(e ast.Expr) bool { _, ok := e.(*ast.SelectorExpr); return ok }

// checkFormatArg checks call's format (argument at) and its arg count.
func checkFormatArg(t *testing.T, fset *token.FileSet, file string, call *ast.CallExpr, at int, consts map[string]ast.Expr, params map[string]bool) {
	t.Helper()
	arg := call.Args[at]
	pos := fset.Position(arg.Pos())
	if id, ok := arg.(*ast.Ident); ok && params[id.Name] && call.Ellipsis.IsValid() {
		return // a wrapper forwarding its own format and args
	}
	if c, ok := arg.(*ast.CallExpr); ok {
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Plain" {
			return // finished text, keyed on itself
		}
	}
	text, ok := constString(arg, consts)
	if !ok {
		if why, listed := formatExceptions[file+":"+exprString(arg)]; listed && why != "" {
			return
		}
		t.Errorf("%s: posting call's format %s is not a literal the extractor can read", pos, exprString(arg))
		return
	}
	key, ok := msgfmt.Key(text)
	if !ok {
		t.Errorf("%s: format %q has a verb msgfmt cannot split", pos, text)
		return
	}
	if verbs, args := strings.Count(strings.ReplaceAll(key, "%%", ""), "%s"), len(call.Args)-at-1; verbs != args && !call.Ellipsis.IsValid() {
		t.Errorf("%s: format %q has %d verbs for %d args", pos, text, verbs, args)
	}
}

// constString is e's value when it is a string literal, literals joined
// with +, or a package constant of one.
func constString(e ast.Expr, consts map[string]ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		a, ok1 := constString(e.X, consts)
		b, ok2 := constString(e.Y, consts)
		return a + b, ok1 && ok2 && e.Op == token.ADD
	case *ast.ParenExpr:
		return constString(e.X, consts)
	case *ast.Ident:
		if v, ok := consts[e.Name]; ok {
			return constString(v, consts)
		}
	case *ast.SelectorExpr: // another package's constant (orders.ReasonDefenders): not followed
	}
	return "", false
}

func exprString(e ast.Expr) string {
	var b strings.Builder
	ast.Walk(walker(func(n ast.Node) {
		if id, ok := n.(*ast.Ident); ok {
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(id.Name)
		}
	}), e)
	return b.String()
}

type walker func(ast.Node)

func (w walker) Visit(n ast.Node) ast.Visitor {
	if n != nil {
		w(n)
	}
	return w
}

// parseDir parses dir's non-test files and collects its package-level
// string constants.
func parseDir(t *testing.T, fset *token.FileSet, dir string) ([]*ast.File, map[string]ast.Expr) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	consts := map[string]ast.Expr{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.CONST {
				for _, s := range gd.Specs {
					vs := s.(*ast.ValueSpec)
					for i, id := range vs.Names {
						if i < len(vs.Values) {
							consts[id.Name] = vs.Values[i]
						}
					}
				}
			}
		}
	}
	return files, consts
}
