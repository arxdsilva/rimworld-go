// Package assetroot finds files the game ships next to it (the assets
// folder). It looks up from the working directory first (running from the
// repository) and then from the executable's directory (a release archive
// started by double-clicking, where the working directory is elsewhere).
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
	var out []string
	if wd, err := os.Getwd(); err == nil {
		out = append(out, wd)
	}
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		out = append(out, filepath.Dir(exe))
	}
	return out
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
