package ebiten

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The object pass's flush guard (GAR-161). While the object pass runs, the
// lit sprites queue as quads in one batch (render/light/lit_batch.go) and
// go out when something else must draw: any raw draw in between (DrawImage,
// DrawTriangles*, vector, gfx's Px* shapes, Text, WorldIcon, a TriBatch
// flush) would land under quads queued before it. So every function the
// object pass can reach that draws raw calls its light layer's Flush (a
// call `….Lit.Flush()`, `lit.Flush()` …) before its first raw draw, or
// joins the batch instead (DrawUnlit), or is listed in litFlushExempt with
// the reason it is safe. A function in litFlushByCaller leaves the flush to
// its callers: a call to it is a raw draw in the caller.
//
// The call graph is by name, from (*Objects).runDraw and the cave mouths'
// draw closures, over render/{world,sprite,creature,overlay,env}: a call
// `x.Foo()` or `pkg.Foo()` reaches every function named Foo there, and a
// call through a Host func field or a root host method of another name
// reaches what it forwards to (litFlushIndirect). That reaches more than runs, never less; an entry
// for a function the pass cannot run says why.

// litFlushRoots are where the object pass starts: the per-draw switch and
// the function whose closures are ArtDraw.Fn (the cave mouths).
var litFlushRoots = []string{"render/world/objects.go:runDraw", "render/world/ng1_art.go:addCaveMouths"}

// litFlushDirs are the layers the object pass runs through.
var litFlushDirs = []string{"render/world/", "render/sprite/", "render/creature/", "render/overlay/", "render/env/"}

// litFlushAround are functions that run the object pass, so a call by
// their name from inside it is another function of that name (Painter.Draw
// is not Pipeline.Draw); the walk does not enter them.
var litFlushAround = map[string]string{
	"render/world/draw_art.go:Draw":    "the frame: it runs DrawArt",
	"render/world/draw_art.go:DrawArt": "the world pass: it runs the object pass",
}

// litFlushExempt are reached functions that draw raw without a Flush, each
// with why the batch cannot be pending (key "file.go:func", as canvasExempt).
var litFlushExempt = map[string]string{}

// litFlushByCaller are raw drawers with no light layer of their own: each
// caller flushes first (a call to one counts as a raw draw in the caller).
var litFlushByCaller = map[string]string{
	"render/creature/burial.go:DrawFlameScreen": "the procedural flame (Host.Flame; the main-menu diorama has no batch)",
	"render/overlay/set.go:drawUnlit":           "overlay's plain sprite draw",
	"render/sprite/fence.go:DrawFenceVector":    "the no-art fence",
}

// litFlushIndirect are the calls the object pass makes through a Host func
// field (wired in layers.go) or a root host method (creature_host.go …)
// that forwards to a function of another name: the call's name and where
// it lands.
var litFlushIndirect = map[string][]string{
	"Flame":         {"render/creature/burial.go:DrawFlameScreen"},
	"Puff":          {"render/world/features_draw.go:DrawPuff"},
	"HostileMarker": {"render/world/labels.go:DrawHostileMarker"},
}

// gfxRawCalls are the gfx canvas calls that draw (the guard's Logical list,
// and the world-pass forms).
var gfxRawCalls = map[string]bool{"WorldIcon": true, "WorldText": true}

// flushScan is one function's calls, Lit flushes and raw draws by line.
type flushScan struct {
	calls   map[string]int // callee name: first line
	flushes []int
	raws    []int
	rawWhat []string
}

// scanFlushFile adds af's functions to out (closures belong to the
// function around them).
func scanFlushFile(fset *token.FileSet, af *ast.File, rel string, out map[string]*flushScan) {
	pkgs := importNames(af)
	for _, d := range af.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		key := rel + ":" + fd.Name.Name
		fs := out[key]
		if fs == nil {
			fs = &flushScan{calls: map[string]int{}}
			out[key] = fs
		}
		ast.Inspect(fd, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			line := fset.Position(c.Pos()).Line
			call := func(name string) {
				if l, ok := fs.calls[name]; !ok || line < l {
					fs.calls[name] = line
				}
			}
			switch fn := c.Fun.(type) {
			case *ast.Ident:
				call(fn.Name)
			case *ast.SelectorExpr:
				call(fn.Sel.Name)
				recv := strings.ToLower(types.ExprString(fn.X))
				switch {
				case fn.Sel.Name == "Flush" && len(c.Args) == 0 && strings.HasSuffix(recv, "lit"):
					fs.flushes = append(fs.flushes, line)
				case fn.Sel.Name == "Flush" && len(c.Args) == 3: // a gfx.TriBatch: one raw triangle draw
					fs.raws, fs.rawWhat = append(fs.raws, line), append(fs.rawWhat, "line "+strconv.Itoa(line)+": TriBatch.Flush")
				}
				if id, ok := fn.X.(*ast.Ident); ok && pkgs[id.Name] == "gfx" && gfxRawCalls[fn.Sel.Name] {
					fs.raws, fs.rawWhat = append(fs.raws, line), append(fs.rawWhat, "line "+strconv.Itoa(line)+": gfx."+fn.Sel.Name)
				}
			}
			return true
		})
	}
}

// objectPassReach is every function the object pass reaches by name, with
// its scan.
func objectPassReach(t *testing.T) (reached []string, scans map[string]*flushScan) {
	t.Helper()
	scans = map[string]*flushScan{}
	fset := token.NewFileSet()
	inDirs := func(rel string) bool {
		for _, d := range litFlushDirs {
			if strings.HasPrefix(rel, d) {
				return true
			}
		}
		return false
	}
	for _, rel := range packageFiles(t, ".") {
		if !inDirs(rel) {
			continue
		}
		src, err := os.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fset, rel, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		scanFlushFile(fset, af, rel, scans)
	}
	// the canvas guard's raw draws (DrawImage & co., vector, text, gfx.Px*)
	// and its logical gfx draws (Text, Icon, FillRect, …) count as raw
	for _, s := range scanPackage(t, ".") {
		fs := scans[s.key()]
		if fs == nil {
			continue
		}
		for _, what := range append(append([]string(nil), s.Raw...), s.Logical...) {
			n, _ := strconv.Atoi(strings.TrimPrefix(what[:strings.Index(what, ":")], "line "))
			fs.raws, fs.rawWhat = append(fs.raws, n), append(fs.rawWhat, what)
		}
	}
	byName := map[string][]string{}
	for k := range scans {
		name := k[strings.LastIndex(k, ":")+1:]
		byName[name] = append(byName[name], k)
	}
	for field, to := range litFlushIndirect {
		byName[field] = append(byName[field], to...)
	}
	// a call to a function that leaves the flush to its callers is a raw
	// draw in the caller
	for _, fs := range scans {
		for name, line := range fs.calls {
			for _, to := range byName[name] {
				if _, ok := litFlushByCaller[to]; ok {
					fs.raws, fs.rawWhat = append(fs.raws, line), append(fs.rawWhat, "line "+strconv.Itoa(line)+": "+name+" (its callers flush)")
				}
			}
		}
	}
	seen := map[string]bool{}
	queue := append([]string(nil), litFlushRoots...)
	for _, r := range litFlushRoots {
		if scans[r] == nil {
			t.Fatalf("root %s is gone: update litFlushRoots", r)
		}
		seen[r] = true
	}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		reached = append(reached, k)
		for name := range scans[k].calls {
			for _, to := range byName[name] {
				if _, around := litFlushAround[to]; !around && !seen[to] {
					seen[to] = true
					queue = append(queue, to)
				}
			}
		}
	}
	sort.Strings(reached)
	return reached, scans
}

// TestObjectPassFlushesBeforeRawDraws (GAR-161): every raw draw the object
// pass can reach comes after its light layer's Flush, joins the batch, or
// is exempt with a reason; stale exemptions fail too.
func TestObjectPassFlushesBeforeRawDraws(t *testing.T) {
	reached, scans := objectPassReach(t)
	isReached := map[string]bool{}
	var bad []string
	for _, k := range reached {
		isReached[k] = true
		fs := scans[k]
		if len(fs.raws) == 0 {
			continue
		}
		if _, byCaller := litFlushByCaller[k]; byCaller {
			continue
		}
		first := slices.Min(fs.raws)
		flushed := len(fs.flushes) > 0 && slices.Min(fs.flushes) < first
		_, exempt := litFlushExempt[k]
		switch {
		case flushed && exempt:
			bad = append(bad, k+": flushes before its raw draws; drop it from litFlushExempt")
		case !flushed && !exempt:
			bad = append(bad, k+": draws raw ("+strings.Join(fs.rawWhat, ", ")+") without the light layer's Flush first")
		}
	}
	for k := range litFlushByCaller {
		if !isReached[k] || len(scans[k].raws) == 0 {
			bad = append(bad, k+": in litFlushByCaller but not reached, gone or not drawing raw")
		}
	}
	for field, to := range litFlushIndirect {
		for _, k := range to {
			if scans[k] == nil {
				bad = append(bad, field+": "+k+" is gone: update litFlushIndirect")
			}
		}
	}
	for k := range litFlushExempt {
		if !isReached[k] {
			bad = append(bad, k+": in litFlushExempt but not reached from the object pass (or gone)")
		} else if len(scans[k].raws) == 0 {
			bad = append(bad, k+": in litFlushExempt but draws nothing raw")
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Error(b)
	}
}
