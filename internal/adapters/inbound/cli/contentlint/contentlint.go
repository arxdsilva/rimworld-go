// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package contentlint is behind cmd/contentlint (PLG-040): it runs the
// whole plugin pipeline headless (no window, no renderer) and prints the
// load order, what each class holds and every problem with its plugin,
// file, def and field. It exits 1 when any problem is an error.
package contentlint

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"rimworld-go/internal/application/modding"
)

// Open makes the plugin source over roots (cmd/contentlint passes the
// modloader adapter).
type Open func(roots ...string) modding.Source

// Main parses args (without the program name) and lints. roots are the
// game's plugin folders, used when -mods is not given; version is the
// build's game version ("dev" skips the version check).
func Main(args []string, out, errOut io.Writer, open Open, roots []string, version string) int {
	fs := flag.NewFlagSet("contentlint", flag.ContinueOnError)
	fs.SetOutput(errOut)
	mods := fs.String("mods", "", "comma-separated plugin folders (default: the game's: "+strings.Join(roots, ", ")+")")
	enable := fs.String("enable", "", "comma-separated plugin ids to enable (default: every installed plugin)")
	game := fs.String("game", version, "game version to check plugins against")
	quiet := fs.Bool("q", false, "print only problems and the summary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *mods != "" {
		roots = split(*mods)
	}
	found, err := open(roots...).Plugins()
	if err != nil {
		fmt.Fprintf(errOut, "contentlint: %v\n", err)
		return 2
	}
	enabled := split(*enable)
	if *enable == "" {
		for _, p := range found {
			enabled = append(enabled, p.Manifest.ID)
		}
	}
	cat, probs := modding.Build(found, enabled, *game)
	probs = append(probs, scriptProblems(found, cat)...) // PLG-060
	if !*quiet {
		report(out, found, cat)
	}
	errs, warns := 0, 0
	for _, p := range probs {
		if p.Warn {
			warns++
		} else {
			errs++
		}
		fmt.Fprintln(out, p.String())
	}
	fmt.Fprintf(out, "%d plugin(s) loaded, %d error(s), %d warning(s)\n", len(cat.Plugins), errs, warns)
	if errs > 0 {
		return 1
	}
	return 0
}

func report(out io.Writer, found []modding.Plugin, cat modding.Catalog) {
	fmt.Fprintf(out, "installed: %d plugin(s) under the given folders\n", len(found))
	fmt.Fprintln(out, "load order:")
	for i, p := range cat.Order.Loaded {
		fmt.Fprintf(out, "  %d. %s (%s)\n", i+1, p.ID, p.Reason)
	}
	for _, id := range cat.Order.Failed {
		fmt.Fprintf(out, "  not loaded: %s\n", id)
	}
	counts := map[string][2]int{} // class -> core, plugins
	for _, d := range cat.Defs {
		c := counts[d.Class]
		if d.Plugin == modding.Core {
			c[0]++
		} else {
			c[1]++
		}
		counts[d.Class] = c
	}
	for _, class := range modding.Classes() {
		c := counts[class]
		fmt.Fprintf(out, "%s: %d core, %d from plugins\n", class, c[0], c[1])
	}
}

func split(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// scriptOptions are the Starlark dialect the game runs scripts in
// (adapters/outbound/starlarkhost.Options; a test keeps them equal).
func scriptOptions() *syntax.FileOptions {
	return &syntax.FileOptions{Set: true, TopLevelControl: true}
}

// scriptProblems compiles the loaded plugins' scripts (PLG-060): a syntax
// or name error is an error at "<file>:<line>".
func scriptProblems(found []modding.Plugin, cat modding.Catalog) []modding.Problem {
	var out []modding.Problem
	for _, p := range found {
		if p.Manifest.ID == modding.Core || !cat.Order.Has(p.Manifest.ID) {
			continue
		}
		for _, b := range p.Scripts {
			if _, _, err := starlark.SourceProgramOptions(scriptOptions(), b.Path, b.Data, func(string) bool { return false }); err != nil {
				out = append(out, scriptProblem(p.Manifest.ID, b.Path, err))
			}
		}
	}
	return out
}

func scriptProblem(plugin, file string, err error) modding.Problem {
	pr := modding.Problem{Plugin: plugin, File: file, Msg: "script: " + err.Error()}
	var se syntax.Error
	var re resolve.ErrorList
	switch {
	case errors.As(err, &se):
		pr.File, pr.Msg = fmt.Sprintf("%s:%d", file, se.Pos.Line), "script: "+se.Msg
	case errors.As(err, &re) && len(re) > 0:
		pr.File, pr.Msg = fmt.Sprintf("%s:%d", file, re[0].Pos.Line), "script: "+re[0].Msg
	}
	return pr
}
