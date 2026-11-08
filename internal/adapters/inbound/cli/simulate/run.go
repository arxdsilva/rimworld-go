package simulate

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"rimworld-go/internal/application/game"
)

// Main runs the tool with argv (program name first), writes the final colony
// state to out and flag errors to errOut, and returns the exit code.
func Main(argv []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet(argv[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	seed := fs.Uint64("seed", 1, "deterministic RNG seed")
	ticks := fs.Int("ticks", 100, "number of simulation ticks to run")
	width := fs.Int("width", 20, "map width in tiles")
	height := fs.Int("height", 20, "map height in tiles")
	if err := fs.Parse(argv[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	print(out, game.NewColony(game.Config{Seed: *seed, Width: *width, Height: *height}), *ticks)
	return 0
}

func print(out io.Writer, g *game.Game, ticks int) {
	g.Step(ticks)
	fmt.Fprintf(out, "tick=%d pawns=%d food=%d wood=%d\n", g.Tick(), len(g.Snapshot()), g.World().Food, g.World().Wood)
	for _, p := range g.Snapshot() {
		fmt.Fprintf(out, "pawn %s: (%d, %d) job=%s hunger=%d rest=%d %s\n", p.Name, p.X, p.Y, p.Job, p.Hunger, p.Rest, p.Status)
	}
}
