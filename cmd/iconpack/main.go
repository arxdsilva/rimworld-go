// Command iconpack writes the rimworld-go app icons (pkg/appicon); run it
// through `make app-icon`, which also builds the macOS .icns with iconutil.
package main

import (
	"flag"
	"fmt"
	"os"

	"rimworld-go/pkg/appicon"
)

func main() {
	pack := flag.String("packaging", "packaging/icons", "packaging output dir")
	rt := flag.String("runtime", "assets/generated/ui/app_icon", "runtime window icon dir")
	flag.Parse()
	if err := appicon.Write(*pack, *rt); err != nil {
		fmt.Fprintln(os.Stderr, "iconpack:", err)
		os.Exit(1)
	}
}
