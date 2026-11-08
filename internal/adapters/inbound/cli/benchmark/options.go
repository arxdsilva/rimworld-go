package benchmark

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type options struct {
	seed       uint64
	sizes      []int
	speeds     []int
	fixtures   []string
	ticks      int
	cache      string
	cpuprofile string
	buildOnly  bool
	hash       bool
}

func parse(args []string) (options, error) {
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	seed := fs.Uint64("seed", 1, "fixture seed")
	sizes := fs.String("sizes", "120,250,500", "map sides, comma-separated")
	speeds := fs.String("speeds", "1,3", "ticks per frame (1 normal, 2 fast, 3 faster), comma-separated")
	fixtures := fs.String("fixtures", "grown,raid", "fixtures: grown (a grown colony) | raid (the same under a 12-raider assault)")
	ticks := fs.Int("ticks", 3000, "ticks to time per run")
	cache := fs.String("cache", os.Getenv("RIMWORLD_FIXTURE_DIR"), "directory keeping built fixtures between runs (default $RIMWORLD_FIXTURE_DIR)")
	cpu := fs.String("cpuprofile", "", "write a CPU profile of the timed ticks to this file")
	buildOnly := fs.Bool("build-only", false, "only build (and cache) the fixtures")
	hash := fs.Bool("hash", false, "print each run's final state hash (compare builds for identical behaviour)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	o := options{seed: *seed, ticks: *ticks, cache: *cache, cpuprofile: *cpu, buildOnly: *buildOnly, hash: *hash}
	var err error
	if o.sizes, err = ints(*sizes); err != nil {
		return o, fmt.Errorf("-sizes: %w", err)
	}
	if o.speeds, err = ints(*speeds); err != nil {
		return o, fmt.Errorf("-speeds: %w", err)
	}
	for _, f := range strings.Split(*fixtures, ",") {
		if f != "grown" && f != "raid" {
			return o, fmt.Errorf("-fixtures: unknown %q", f)
		}
		o.fixtures = append(o.fixtures, f)
	}
	if o.ticks < 1 {
		return o, fmt.Errorf("-ticks must be positive")
	}
	return o, nil
}

func ints(s string) ([]int, error) {
	var out []int
	for _, f := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("bad value %q", f)
		}
		out = append(out, n)
	}
	return out, nil
}
