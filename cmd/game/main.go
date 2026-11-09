// Command game is the rimworld-go client. It parses the command line
// (internal/adapters/inbound/cli/game) and runs the composition root
// (internal/compose/game) on the Ebitengine screen. Release builds set the
// version with -ldflags "-X rimworld-go/internal/platform/buildinfo.Version=v1.2.3".
// -version and -selfcheck answer and exit before any window opens.
package main

import (
	"os"

	cligame "rimworld-go/internal/adapters/inbound/cli/game"
	"rimworld-go/internal/adapters/inbound/ebiten/display"
	composegame "rimworld-go/internal/compose/game"
	"rimworld-go/internal/platform/buildinfo"
)

func main() {
	opts, err := cligame.Parse(os.Args)
	if err != nil {
		os.Exit(cligame.ExitCode(err))
	}
	if code, done := composegame.Report(opts, buildinfo.Version, composegame.ConsoleOut(os.Stdout)); done {
		os.Exit(code)
	}
	composegame.Run(opts, buildinfo.Version, display.Screen{})
}
