// Command atlasgen packs one sprite family into texture atlas pages; the
// logic lives in internal/adapters/inbound/cli/atlasgen.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/atlasgen"
)

func main() { os.Exit(atlasgen.Main(os.Args[1:], os.Stdout, os.Stderr)) }
