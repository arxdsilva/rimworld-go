// Command normalgen writes a normal map next to every world sprite (ART §6.1);
// the logic lives in internal/adapters/inbound/cli/normalgen.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/normalgen"
)

func main() { os.Exit(normalgen.Main(os.Args[1:], os.Stdout, os.Stderr)) }
