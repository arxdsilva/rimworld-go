// Package openfolder shows a folder in the system's file browser. The path
// is passed as one argument to the platform's opener; no shell is involved.
package openfolder

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// command is the opener for an OS: the program and its arguments.
func command(goos, path string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{path}, nil
	case "windows":
		return "explorer", []string{path}, nil
	case "linux", "freebsd", "openbsd", "netbsd":
		return "xdg-open", []string{path}, nil
	}
	return "", nil, fmt.Errorf("openfolder: no file browser known for %s", goos)
}

// Open shows path in the file browser, creating it first when it is missing
// (a player who has saved nothing still gets a folder to drop files in). It
// starts the browser and returns without waiting.
func Open(path string) error {
	if path == "" {
		return errors.New("openfolder: no folder")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("openfolder: %w", err)
	}
	name, args, err := command(runtime.GOOS, path)
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("openfolder: %w", err)
	}
	go func() { _ = cmd.Wait() }() // reap it; explorer exits 1 on success
	return nil
}
