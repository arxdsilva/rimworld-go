package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/cmd/mapgen/render"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
)

func parse(t *testing.T, args ...string) Options {
	t.Helper()
	o, err := ParseArgs(args, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("ParseArgs(%v): %v", args, err)
	}
	return o
}

func TestParseArgsFlags(t *testing.T) {
	o := parse(t, "-out", "x", "-seed", "7", "-biome", "desert", "-hills", "mountainous", "-size", "100",
		"-river", "large:W>E", "-coast", "W,NW", "-lakes", "3", "-caves", "-stones", "granite,marble",
		"-road", "E,S", "-settled")
	if o.Seed != 7 || o.Size != 100 || o.Hills != site.Mountainous || o.Lakes != 3 || !o.Caves || !o.Settled {
		t.Fatalf("scalars: %+v", o)
	}
	if o.River != (site.RiverPath{Size: site.LargeRiver, In: 6, Out: 2}) {
		t.Fatalf("river %+v", o.River)
	}
	if !o.Coast.Has(6) || !o.Coast.Has(7) || o.Coast.Has(0) || !o.Road.Has(2) || !o.Road.Has(4) {
		t.Fatalf("edges coast=%b road=%b", o.Coast, o.Road)
	}
	if len(o.Stones) != 2 || o.Stones[0] != geology.Granite || o.Stones[1] != geology.Marble {
		t.Fatalf("stones %v", o.Stones)
	}
	if o.Scale != 2 {
		t.Fatalf("default scale %d", o.Scale)
	}
	s := o.Site()
	if s.Hills != site.Mountainous || s.River.Size != site.LargeRiver || len(s.Stones) != 2 {
		t.Fatalf("site %+v", s)
	}
	if parse(t, "-out", "x", "-size", "300").Scale != 1 {
		t.Fatal("large maps default to scale 1")
	}
	if parse(t, "-out", "x", "-scale", "3").Scale != 3 {
		t.Fatal("explicit scale")
	}
	if r := parse(t, "-out", "x", "-river", "creek").River; r.Size != site.Creek || r.In != 0 || r.Out != 4 {
		t.Fatalf("default river path %+v", r)
	}
}

func TestParseArgsErrors(t *testing.T) {
	bad := [][]string{
		{},
		{"-out", "x", "-biome", "nope"},
		{"-out", "x", "-hills", "tall"},
		{"-out", "x", "-size", "2"},
		{"-out", "x", "-size", "5000"},
		{"-out", "x", "-river", "stream"},
		{"-out", "x", "-river", "river:NS"},
		{"-out", "x", "-river", "river:N>Q"},
		{"-out", "x", "-river", "river:Q>N"},
		{"-out", "x", "-coast", "Z"},
		{"-out", "x", "-road", "N,Q"},
		{"-out", "x", "-stones", "cheese"},
		{"-out", "x", "-lakes", "300"},
		{"-out", "x", "-scale", "99"},
		{"-out", "x", "-grid", "-1"},
		{"-out", "x", "-sweep", "size"},
		{"-out", "x", "-tile", "nocolon"},
		{"-out", "x", "-tile", "a:b"},
		{"-out", "x", "-bogus"},
		{"-out", "x", "extra"},
	}
	for _, a := range bad {
		if _, err := ParseArgs(a, &bytes.Buffer{}); err == nil {
			t.Errorf("ParseArgs(%v) accepted bad input", a)
		}
	}
}

func TestParseTile(t *testing.T) {
	text, id, err := parseTile("amber:hollow:12")
	if err != nil || text != "amber:hollow" || id != 12 {
		t.Fatalf("got %q %d %v", text, id, err)
	}
	if _, _, err := parseTile(":5"); err == nil {
		t.Fatal("empty seed text accepted")
	}
	if _, _, err := parseTile("a:-1"); err == nil {
		t.Fatal("negative tile accepted")
	}
}

func decodeSize(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

func TestRunSingleWritesEverything(t *testing.T) {
	dir := t.TempDir()
	o := parse(t, "-out", dir, "-size", "48", "-scale", "2", "-seed", "5", "-river", "river:N>S", "-coast", "W", "-lakes", "1", "-caves")
	var out bytes.Buffer
	if err := Run(o, &out); err != nil {
		t.Fatal(err)
	}
	for _, n := range render.Names {
		if w, h := decodeSize(t, filepath.Join(dir, n+".png")); w != 96 || h != 96 {
			t.Errorf("%s.png is %dx%d, want 96x96", n, w, h)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "metrics.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mf MetricsFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		t.Fatal(err)
	}
	if mf.Seed != 5 || mf.Size != 48 || len(mf.Timings) == 0 || mf.Timings[0].Step != "elevation" || mf.River == "" {
		t.Fatalf("metrics.json: %+v", mf)
	}
	if _, err := os.Stat(filepath.Join(dir, "summary.txt")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "mapgen seed=5") {
		t.Fatalf("summary: %q", out.String())
	}
}

func TestRunGridAndSweep(t *testing.T) {
	dir := t.TempDir()
	if err := Run(parse(t, "-out", dir, "-size", "24", "-scale", "1", "-grid", "6"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	w, h := render.SheetSize(6, 24, 24)
	if gw, gh := decodeSize(t, filepath.Join(dir, "grid.png")); gw != w || gh != h || w != 4*24+3*render.SheetGap {
		t.Fatalf("grid %dx%d, want %dx%d", gw, gh, w, h)
	}
	dir = t.TempDir()
	if err := Run(parse(t, "-out", dir, "-size", "24", "-scale", "2", "-sweep", "hills"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	w, h = render.SheetSize(4, 48, 48)
	if gw, gh := decodeSize(t, filepath.Join(dir, "sweep_hills.png")); gw != w || gh != h {
		t.Fatalf("sweep %dx%d, want %dx%d", gw, gh, w, h)
	}
}

func TestRunTile(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a planet")
	}
	dir := t.TempDir()
	o := parse(t, "-out", dir, "-size", "24", "-scale", "1", "-tile", "amber-hollow:100")
	if err := Run(o, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "metrics.json"))
	if !strings.Contains(string(raw), `"tile": "amber-hollow:100"`) {
		t.Fatalf("metrics lacks tile: %s", raw)
	}
	bad := parse(t, "-out", dir, "-size", "24", "-tile", "amber-hollow:99999999")
	if err := Run(bad, &bytes.Buffer{}); err == nil {
		t.Fatal("out-of-range tile accepted")
	}
}

func TestRunErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	os.WriteFile(file, nil, 0o644)
	if err := Run(parse(t, "-out", filepath.Join(file, "sub"), "-size", "24"), &bytes.Buffer{}); err == nil {
		t.Fatal("unwritable out dir accepted")
	}
	if err := Run(Options{Out: t.TempDir(), Size: 0, Scale: 1}, &bytes.Buffer{}); err == nil {
		t.Fatal("zero size accepted")
	}
}

func TestMainRun(t *testing.T) {
	var o, e bytes.Buffer
	if c := run([]string{"-out", t.TempDir(), "-size", "24", "-scale", "1"}, &o, &e); c != 0 {
		t.Fatalf("exit %d: %s", c, e.String())
	}
	if c := run([]string{}, &o, &e); c != 2 {
		t.Fatalf("missing -out exit %d", c)
	}
	if c := run([]string{"-h"}, &o, &e); c != 2 {
		t.Fatalf("-h exit %d", c)
	}
	file := filepath.Join(t.TempDir(), "f")
	os.WriteFile(file, nil, 0o644)
	if c := run([]string{"-out", filepath.Join(file, "x"), "-size", "24"}, &o, &e); c != 1 {
		t.Fatalf("run error exit %d", c)
	}
}
