package lang

import (
	"go/ast"
	"slices"
	"strings"
	"testing"

	"rimworld-go/pkg/msgfmt"
)

// The simulation's messages and letters reach the HUD as msgfmt.Text
// (lang.Msg): the catalog keys their formats, the arguments by exact
// key, and the titles by exact key. simKeys finds those keys in the game
// tree so the English template holds them.

// What a poster's argument is to the catalog.
const (
	roleFormat = iota + 1 // a printf format: keyed as msgfmt.Key(format)
	roleTitle             // a letter or threat title: keyed as is
	rolePlain             // finished letter text: keyed as msgfmt.Plain(text)
	roleArg               // a format argument: the HUD translates it by exact key
)

// poster is a function or method that posts a message or letter: the
// role of each argument it keys, and from where its variadic format
// arguments start (-1: none).
type poster struct {
	at       map[int]int
	argsFrom int
}

// seedPosters are the game's own constructors (climate.go, world_state.go;
// game is the root package's import path). Hosts and helpers that forward a
// parameter to one of them are found by discoverPosters, so a sub-package's
// h.Note("…") is keyed too.
func seedPosters(game string) map[string]poster {
	return map[string]poster{
		funcKey(game, "letterf"): {at: map[int]int{2: roleTitle, 4: roleFormat}, argsFrom: 5},
		"method:message/*":       {at: map[int]int{2: roleFormat}, argsFrom: 3},
		"method:threat/*":        {at: map[int]int{0: roleTitle, 3: roleFormat}, argsFrom: 4},
		"method:worldNote/*":     {at: map[int]int{0: roleFormat}, argsFrom: 1},
		methodKey("letter", 4):   {at: map[int]int{1: roleTitle, 2: rolePlain}, argsFrom: -1},
	}
}

// callTargets are the keys call may resolve to, exact first.
func callTargets(f *srcFile, call *ast.CallExpr) []string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return []string{funcKey(f.path, fn.Name)}
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			if ip, ok := f.imports[x.Name]; ok {
				return []string{funcKey(ip, fn.Sel.Name)}
			}
		}
		return []string{methodKey(fn.Sel.Name, len(call.Args)), "method:" + fn.Sel.Name + "/*"}
	}
	return nil
}

func posterFor(f *srcFile, call *ast.CallExpr, posters map[string]poster) (poster, bool) {
	for _, k := range callTargets(f, call) {
		if p, ok := posters[k]; ok {
			return p, true
		}
	}
	return poster{}, false
}

// roleOf is the role of call's argument i under p (0: none).
func (p poster) roleOf(i int) int {
	if r, ok := p.at[i]; ok {
		return r
	}
	if p.argsFrom >= 0 && i >= p.argsFrom {
		return roleArg
	}
	return 0
}

// discoverPosters adds every game function that forwards one of its
// parameters to a poster's keyed argument, until nothing new turns up.
func discoverPosters(files []*srcFile, posters map[string]poster) {
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, d := range f.ast.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil && addForwarder(f, fd, posters) {
					changed = true
				}
			}
		}
	}
}

func addForwarder(f *srcFile, fd *ast.FuncDecl, posters map[string]poster) bool {
	params, n, variadic := map[string]int{}, 0, false
	for _, fld := range fd.Type.Params.List {
		_, variadic = fld.Type.(*ast.Ellipsis)
		for _, nm := range fld.Names {
			params[nm.Name] = n
			n++
		}
		if len(fld.Names) == 0 {
			n++
		}
	}
	if len(params) == 0 {
		return false
	}
	key := funcKey(f.path, fd.Name.Name)
	if fd.Recv != nil {
		key = methodKey(fd.Name.Name, n)
		if variadic {
			key = "method:" + fd.Name.Name + "/*"
		}
	}
	cur, had := posters[key]
	if !had {
		cur = poster{at: map[int]int{}, argsFrom: -1}
	}
	added := false
	ast.Inspect(fd.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		p, ok := posterFor(f, call, posters)
		if !ok {
			return true
		}
		for i, a := range call.Args {
			role := p.roleOf(i)
			id, isID := a.(*ast.Ident)
			if role == 0 || !isID {
				continue
			}
			j, ok := params[id.Name]
			if !ok {
				continue
			}
			if variadic && j == n-1 && call.Ellipsis.IsValid() {
				if cur.argsFrom != j {
					cur.argsFrom, added = j, true
				}
				continue
			}
			if _, has := cur.at[j]; !has {
				cur.at[j], added = role, true
			}
		}
		return true
	})
	if added {
		posters[key] = cur
	}
	return added
}

// simKeys is every catalog key the game tree posts: literal (or same-package
// constant) formats, titles, plain letter texts and literal arguments of the
// posters; msgfmt.Of/Plain calls and MessageView/LetterView titles written
// directly; and game/sense's Why* refusal reasons (gear tab tips).
func simKeys(files []*srcFile, game string) map[string]bool {
	posters := seedPosters(game)
	discoverPosters(files, posters)
	consts := stringConsts(files)
	keys := map[string]bool{}
	for _, f := range files {
		lit := func(e ast.Expr) (string, bool) {
			if s, ok := strLit(e); ok {
				return s, true
			}
			if id, ok := e.(*ast.Ident); ok {
				s, ok := consts[f.pkg][id.Name]
				return s, ok
			}
			return "", false
		}
		add := func(role int, e ast.Expr) {
			s, ok := lit(e)
			if !ok {
				return
			}
			switch role {
			case roleFormat:
				s, ok = msgfmt.Key(s)
			case rolePlain:
				s = msgfmt.Plain(s)
			}
			if ok && hasLetter(s) {
				keys[s] = true
			}
		}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if isMsgfmt(f, x, "Of") && len(x.Args) > 0 {
					add(roleFormat, x.Args[0])
					for _, a := range x.Args[1:] {
						add(roleArg, a)
					}
				} else if isMsgfmt(f, x, "Plain") && len(x.Args) == 1 {
					add(rolePlain, x.Args[0])
				} else if p, ok := posterFor(f, x, posters); ok {
					for i, a := range x.Args {
						if r := p.roleOf(i); r != 0 {
							add(r, a)
						}
					}
				}
			case *ast.CompositeLit:
				if isView(x.Type) {
					for _, el := range x.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Title" {
								add(roleTitle, kv.Value)
							}
						}
					}
				}
			}
			return true
		})
	}
	for name, v := range consts["sense"] {
		if strings.HasPrefix(name, "Why") && hasLetter(v) {
			keys[v] = true
		}
	}
	return keys
}

func isMsgfmt(f *srcFile, call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && strings.HasSuffix(f.imports[x.Name], "/pkg/msgfmt")
}

// isView reports whether a composite literal builds a MessageView or a
// LetterView (game.MessageView from outside the package).
func isView(e ast.Expr) bool {
	name := ""
	switch x := e.(type) {
	case *ast.Ident:
		name = x.Name
	case *ast.SelectorExpr:
		name = x.Sel.Name
	}
	return name == "MessageView" || name == "LetterView"
}

// TestSimKeysSeeEveryForm: the posting shapes simKeys reads.
func TestSimKeysSeeEveryForm(t *testing.T) {
	src := `package x
import (
	"fmt"
	"rimworld-go/pkg/msgfmt"
)
const fmtTamed = "%s tamed a %s."
func (g *Game) message(x, y int, format string, args ...any) {}
func (g *Game) threat(title string, x, y int, format string, args ...any) {}
func (g *Game) worldNote(format string, args ...any) { g.message(0, 0, format, args...) }
func (g *Game) letter(kind, title, text string, focus int) {}
func letterf(g *Game, kind, title string, focus int, format string, args ...any) {}
type host struct{ g *Game }
func (h host) Note(text string) { h.g.worldNote("%s", text) }
func (h host) Letter(danger bool, title, text string, focus int) { h.g.letter("info", title, text, focus) }
func (h host) Threat(title string, x, y int, text string) { h.g.threat(title, x, y, "%s", text) }
func f(g *Game, h host, name string) {
	g.message(1, 2, "%s ate %d meals (%.1f%%).", name, 3, 4.5)
	g.message(1, 2, fmtTamed, name, "boar")
	g.message(1, 2, "%s", name)
	g.message(1, 2, dynamic, name)
	g.worldNote("The caravan has left.")
	g.threat("Raid", 1, 2, "Raiders from %s.", name)
	g.letter("good", "Wanderers", "Two wanderers joined: 100% sure.", 0)
	g.letter("good", "Built", fmt.Sprintf("%s finished.", name), 0)
	h.Note("Nobody can join the caravan.")
	h.Letter(false, "Caravan arrived", text, 0)
	h.Threat("Manhunters", 1, 2, "Animals went mad.")
	g.post(MessageView{Title: "Direct", Msg: msgfmt.Of("Direct %d post", 2)})
	_ = msgfmt.Make(msgfmt.Plain("Plain 5%"), nil)
	letterf(g, "good", "Inspired", 0, "%s is inspired: %s.", name, "creativity")
}`
	f := parseOne(t, src)
	f.imports["msgfmt"] = "rimworld-go/pkg/msgfmt"
	got := simKeys([]*srcFile{f}, f.path)
	want := []string{"%s ate %s meals (%s%%).", "%s tamed a %s.", "boar", "The caravan has left.", "Raid", "Raiders from %s.",
		"Wanderers", "Two wanderers joined: 100%% sure.", "Built", "Nobody can join the caravan.", "Caravan arrived",
		"Manhunters", "Animals went mad.", "Direct", "Direct %s post", "Plain 5%%",
		"Inspired", "%s is inspired: %s.", "creativity"}
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Fatalf("keys\n%q\nwant\n%q", keys, want)
	}
}
