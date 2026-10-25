package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
)

// Options is the parsed command line.
type Options struct {
	Seed    uint64
	Biome   biome.ID
	Hills   site.Hills
	Size    int
	River   site.RiverPath
	Coast   site.Dir8Set
	Lakes   uint8
	Caves   bool
	Stones  []geology.Stone
	Road    site.Dir8Set
	Settled bool
	Out     string
	Scale   int
	Grid    int
	Sweep   string // "" or "hills"
	Tile    string // "" or "<seedText>:<tileID>"
}

var dirNames = [8]string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}

func parseDir(s string) (site.Dir8, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	for i, n := range dirNames {
		if n == s {
			return site.Dir8(i), nil
		}
	}
	return 0, fmt.Errorf("unknown edge %q (want N, NE, E, SE, S, SW, W or NW)", s)
}

func parseEdges(s string) (site.Dir8Set, error) {
	var set site.Dir8Set
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	for _, part := range strings.Split(s, ",") {
		d, err := parseDir(part)
		if err != nil {
			return 0, err
		}
		set |= 1 << d
	}
	return set, nil
}

// parseRiver reads "creek", "river:N>S" or "large:W>E" (default N>S).
func parseRiver(s string) (site.RiverPath, error) {
	if s == "" {
		return site.RiverPath{}, nil
	}
	kind, path, _ := strings.Cut(s, ":")
	var p site.RiverPath
	switch strings.ToLower(kind) {
	case "creek":
		p.Size = site.Creek
	case "river":
		p.Size = site.River
	case "large", "large_river", "largeriver":
		p.Size = site.LargeRiver
	default:
		return p, fmt.Errorf("unknown river size %q (want creek, river or large)", kind)
	}
	p.In, p.Out = 0, 4
	if path != "" {
		in, out, ok := strings.Cut(path, ">")
		if !ok {
			return p, fmt.Errorf("river path %q: want IN>OUT like N>S", path)
		}
		var err error
		if p.In, err = parseDir(in); err != nil {
			return p, err
		}
		if p.Out, err = parseDir(out); err != nil {
			return p, err
		}
	}
	return p, nil
}

func parseHills(s string) (site.Hills, error) {
	switch strings.ToLower(s) {
	case "default":
		return site.HillsDefault, nil
	case "flat":
		return site.Flat, nil
	case "small":
		return site.SmallHills, nil
	case "large":
		return site.LargeHills, nil
	case "mountainous":
		return site.Mountainous, nil
	}
	return 0, fmt.Errorf("unknown hills %q (want flat, small, large or mountainous)", s)
}

func parseStones(s string) ([]geology.Stone, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []geology.Stone
	for _, n := range strings.Split(s, ",") {
		st, ok := geology.ParseStone(strings.ToLower(strings.TrimSpace(n)))
		if !ok {
			return nil, fmt.Errorf("unknown stone %q", n)
		}
		out = append(out, st)
	}
	return out, nil
}

// parseTile splits "<seedText>:<tileID>" at the last colon.
func parseTile(s string) (text string, tile int, err error) {
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return "", 0, fmt.Errorf("tile %q: want <seedText>:<tileID>", s)
	}
	tile, err = strconv.Atoi(s[i+1:])
	if err != nil || tile < 0 {
		return "", 0, fmt.Errorf("tile %q: bad tile id", s)
	}
	return s[:i], tile, nil
}

// ParseArgs parses the command line; usage and flag errors go to errw.
func ParseArgs(args []string, errw io.Writer) (Options, error) {
	var o Options
	fs := flag.NewFlagSet("mapgen", flag.ContinueOnError)
	fs.SetOutput(errw)
	seed := fs.Uint64("seed", 1001, "colony seed")
	biomeKey := fs.String("biome", "temperate_forest", "biome key (e.g. temperate_forest)")
	hills := fs.String("hills", "large", "flat|small|large|mountainous")
	size := fs.Int("size", 250, "map side in cells")
	river := fs.String("river", "", "creek|river|large, optionally :IN>OUT (e.g. river:N>S)")
	coast := fs.String("coast", "", "ocean edges, e.g. W,NW")
	lakes := fs.Int("lakes", 0, "lake count 0..255")
	caves := fs.Bool("caves", false, "cave tunnels")
	stones := fs.String("stones", "", "permitted stones, e.g. granite,marble")
	road := fs.String("road", "", "road edges, e.g. E,S")
	settled := fs.Bool("settled", false, "a settlement nearby (more ruins)")
	out := fs.String("out", "", "output directory (required)")
	scale := fs.Int("scale", 0, "pixel upscale (default 2 up to 250 cells, else 1)")
	grid := fs.Int("grid", 0, "contact sheet of N consecutive seeds (4 per row)")
	sweep := fs.String("sweep", "", "sweep a parameter: hills")
	tile := fs.String("tile", "", "<seedText>:<tileID>: generate that world tile's site")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() > 0 {
		return o, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *out == "" {
		return o, errors.New("-out is required")
	}
	if *size < 8 || *size > 1000 {
		return o, fmt.Errorf("-size %d out of range 8..1000", *size)
	}
	if *lakes < 0 || *lakes > 255 {
		return o, fmt.Errorf("-lakes %d out of range 0..255", *lakes)
	}
	if *scale < 0 || *scale > 16 {
		return o, fmt.Errorf("-scale %d out of range 0..16", *scale)
	}
	if *grid < 0 || *grid > 64 {
		return o, fmt.Errorf("-grid %d out of range 0..64", *grid)
	}
	if *sweep != "" && *sweep != "hills" {
		return o, fmt.Errorf("-sweep %q: only hills is supported", *sweep)
	}
	if *tile != "" {
		if _, _, err := parseTile(*tile); err != nil {
			return o, err
		}
	}
	id, ok := biome.Parse(*biomeKey)
	if !ok {
		return o, fmt.Errorf("unknown biome %q", *biomeKey)
	}
	var err error
	o = Options{Seed: *seed, Biome: id, Size: *size, Lakes: uint8(*lakes), Caves: *caves,
		Settled: *settled, Out: *out, Scale: *scale, Grid: *grid, Sweep: *sweep, Tile: *tile}
	if o.Hills, err = parseHills(*hills); err != nil {
		return o, err
	}
	if o.River, err = parseRiver(*river); err != nil {
		return o, err
	}
	if o.Coast, err = parseEdges(*coast); err != nil {
		return o, err
	}
	if o.Road, err = parseEdges(*road); err != nil {
		return o, err
	}
	if o.Stones, err = parseStones(*stones); err != nil {
		return o, err
	}
	if o.Scale == 0 {
		o.Scale = 1
		if o.Size <= 250 {
			o.Scale = 2
		}
	}
	return o, nil
}

// Site builds the local-map parameters the options describe.
func (o Options) Site() site.Site {
	return site.Site{
		Biome: o.Biome, Climate: biome.DefOf(o.Biome).Climate, Hills: o.Hills,
		River: o.River, Coast: o.Coast, Lakes: o.Lakes, Caves: o.Caves,
		Stones: o.Stones, Road: o.Road, Settled: o.Settled,
	}
}
