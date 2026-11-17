// Command contentlint checks plugins without starting the game (PLG-040);
// see cli/contentlint.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/contentlint"
	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/platform/buildinfo"
)

func main() {
	open := func(roots ...string) modding.Source { return modloader.New(roots...) }
	os.Exit(contentlint.Main(os.Args[1:], os.Stdout, os.Stderr, open, modloader.DefaultRoots(), buildinfo.Version))
}
