// Package assetroot finds files the game ships next to it (the assets
// folder). It looks up from the working directory first (running from the
// repository), then from the executable's directory (a release archive
// started by double-clicking, where the working directory is elsewhere).
// A macOS app bundle keeps the executable in X.app/Contents/MacOS and the
// assets in X.app/Contents/Resources, so that folder is tried before the
// executable's own. The executable's path is resolved through symlinks
// first (/usr/bin/rimworld-go → /opt/rimworld-go/rimworld-go).
package assetroot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxUp bounds how many parent directories are tried from each start.
const maxUp = 12

// Find returns the first existing path rel under a start directory or one
// of its parents; dir says whether it must be a directory (else a file).
func Find(rel string, dir bool) (string, error) {
	for _, start := range starts() {
		if p, ok := findUp(start, rel, dir); ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found above the working directory or the executable", filepath.ToSlash(rel))
}

// FindParent is Find for a marker below a root (e.g. "assets" holding
// "metadata/audio/audio.json"): it returns the root.
func FindParent(root, marker string, dir bool) (string, error) {
	p, err := Find(filepath.Join(root, marker), dir)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(p, string(filepath.Separator)+filepath.Clean(marker)), nil
}

func starts() []string {
	wd, _ := os.Getwd()       // "" on error
	exe, _ := os.Executable() // "" on error
	return startsFrom(wd, exe)
}

// startsFrom is the search order for working directory wd and executable
// exe ("" when unknown): wd, then a bundle's Resources folder when the
// real executable sits in a MacOS folder, then the executable's folder.
func startsFrom(wd, exe string) []string {
	var out []string
	if wd != "" {
		out = append(out, wd)
	}
	if exe == "" {
		return out
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dir := filepath.Dir(exe)
	if filepath.Base(dir) == "MacOS" {
		out = append(out, filepath.Join(filepath.Dir(dir), "Resources"))
	}
	return append(out, dir)
}

func findUp(start, rel string, dir bool) (string, bool) {
	d := start
	for i := 0; i < maxUp; i++ {
		cand := filepath.Join(d, rel)
		if st, err := os.Stat(cand); err == nil && st.IsDir() == dir {
			return cand, true
		}
		up := filepath.Dir(d)
		if up == d {
			break
		}
		d = up
	}
	return "", false
}
