package lang

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

// guardFails turns TestUITextIsTranslatable from a report into a failure.
// OPT-3 flips it once every package draws its text through T/Tf.
const guardFails = false

var viaNames = [viaKinds]string{"call", "field", "var", "return"}

// TestUITextIsTranslatable reports, per package, the string literals the
// client draws without T/Tf (OPT-3 is planned from it). It fails only once
// guardFails is flipped; lang/exempt.txt lists ids and developer text.
//
// The count is a floor: the guard reads syntax only, so text built far
// from where it is drawn is missed. "all" is every letter-bearing literal
// in the package, the ceiling.
func TestUITextIsTranslatable(t *testing.T) {
	t.Parallel()
	files := parseClient(t)
	client := module(t, repoRoot(t)) + "/" + ebitenRel
	sinks := seedSinks(client)
	discoverSinks(files, sinks)
	ex := loadExempt(t)
	type count struct {
		via    [viaKinds]int
		open   int
		exempt int
	}
	per := map[string]*count{}
	var open []rawText
	for _, r := range untranslated(files, sinks) {
		c := per[r.pkg]
		if c == nil {
			c = &count{}
			per[r.pkg] = c
		}
		if ex.match(r) {
			c.exempt++
			continue
		}
		c.via[r.via]++
		c.open++
		open = append(open, r)
	}
	all := allLiterals(files)
	pkgs := make([]string, 0, len(per))
	for p := range per {
		pkgs = append(pkgs, p)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		if a, b := per[pkgs[i]].open, per[pkgs[j]].open; a != b {
			return a > b
		}
		return pkgs[i] < pkgs[j]
	})
	var b strings.Builder
	var tot count
	tall := 0
	row := "%-16s %6d %6d %6d %6d %6d %6d %6d\n"
	fmt.Fprintf(&b, "untranslated UI text (%d sinks):\n%-16s %6s %6s %6s %6s %6s %6s %6s\n",
		len(sinks), "package", "open", "call", "field", "var", "return", "exempt", "all")
	for _, p := range pkgs {
		c := per[p]
		fmt.Fprintf(&b, row, p, c.open, c.via[viaCall], c.via[viaField], c.via[viaVar], c.via[viaReturn], c.exempt, all[p])
		tot.open += c.open
		tot.exempt += c.exempt
		for i := range tot.via {
			tot.via[i] += c.via[i]
		}
		tall += all[p]
	}
	fmt.Fprintf(&b, row, "TOTAL", tot.open, tot.via[viaCall], tot.via[viaField], tot.via[viaVar], tot.via[viaReturn], tot.exempt, tall)
	t.Log(b.String())
	// LANG_GUARD_LIST=hud (a package, or "all") lists that package's sites.
	if want := os.Getenv("LANG_GUARD_LIST"); want != "" {
		for _, r := range open {
			if want == "all" || want == r.pkg {
				t.Logf("%s:%d [%s] %q", r.file, r.line, viaNames[r.via], r.text)
			}
		}
	}
	for _, s := range ex.unused() {
		t.Logf("exempt.txt:%d matches nothing: %s", s.line, s.raw)
	}
	if guardFails && len(open) > 0 {
		for i, r := range open {
			if i == 40 {
				t.Errorf("… and %d more", len(open)-40)
				break
			}
			t.Errorf("%s:%d draws %q without T/Tf", r.file, r.line, r.text)
		}
	}
}

// TestGuardSeesEveryForm: sinks, wrappers, fields, locals, constants,
// drawn returns and the shapes a literal reaches them in.
func TestGuardSeesEveryForm(t *testing.T) {
	src := `package x
import (
	"fmt"
	"example.com/c/gfx"
	"example.com/c/uikit"
)
const heading = "Heading"
func drawText(dst any, s string, x, y float64) { gfx.Text(dst, s, x, y, st) }
func lines(r any, ls ...string) { uikit.TabRects(r, ls) }
func modeName(m int) string {
	if m == 0 {
		return "Mode zero"
	}
	f := func() string { return "Closure" }
	return other(m) + f()
}
func other(int) string { return "Other mode" }
func (k kind) Label() string { return "Kind label" }
func (k kind) Id() string { return "kind.id" }
func f(k *uikit.Kit, tx tr) {
	gfx.Text(dst, "Plain", 0, 0, st)
	gfx.Text(dst, tx.T("Wrapped"), 0, 0, st)
	gfx.Text(dst, "Food: "+n, 0, 0, st)
	gfx.Text(dst, fmt.Sprintf("%d days of %s", n, "rice"), 0, 0, st)
	gfx.Text(dst, "%d", 0, 0, st)
	drawText(dst, "Via helper", 0, 0)
	k.Button("btn.id", r, "Press", o)
	k.Tabs("tabs", r, []string{"One", tx.T("Two")}, &sel, o)
	_ = uikit.ButtonSpec{ID: "spec.id", Label: "Spec"}
	k.Button("btn.id", r, tx.Tf("%d left", n), o)
	lines(r, "First", "Second")
	label := "Local"
	if n > 0 {
		label = "Other local"
	}
	gfx.Text(dst, label, 0, 0, st)
	gfx.Text(dst, label, 0, 0, st)
	gfx.Text(dst, heading, 0, 0, st)
	gfx.Text(dst, modeName(n), 0, 0, st)
	gfx.Text(dst, kd.Label(), 0, 0, st)
	gfx.Text(dst, name, 0, 0, st)
}`
	f := parseOne(t, src)
	sinks := seedSinks("example.com/c")
	discoverSinks([]*srcFile{f}, sinks)
	var got []string
	for _, r := range untranslated([]*srcFile{f}, sinks) {
		got = append(got, r.text)
	}
	want := []string{"Spec", "Plain", "Food: ", "%d days of %s", "rice", "Via helper", "Press", "One",
		"First", "Second", "Local", "Other local", "Heading", "Mode zero", "Other mode", "Kind label"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
