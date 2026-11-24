// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package logfile

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLogFileKeepsTheLastRunAsPrev(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "logs")
	for _, run := range []string{"first run\n", "second run\n"} {
		f, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(run); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]string{name: "second run\n", prevName: "first run\n"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Errorf("%s = %q, want %q", name, b, want)
		}
	}
}

func TestOpenCreatesPrivateDirAndFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows has no unix mode bits")
	}
	dir := filepath.Join(t.TempDir(), "logs")
	f, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := di.Mode().Perm() & 0o027; got != 0 {
		t.Errorf("dir mode %v has group-write or other bits", di.Mode().Perm())
	}
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm() & 0o077; got != 0 {
		t.Errorf("file mode %v has group or other bits", fi.Mode().Perm())
	}
}

// Start tees the logger to game.log; stop puts stderr back and closes the file.
// It changes the process-wide logger, so it is not parallel.
func TestStartStopRestoresStderrAndClosesTheFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("APPDATA", filepath.Join(home, "cfg"))
	prev := log.Writer()
	t.Cleanup(func() { log.SetOutput(prev) })

	stop := Start()
	tee := log.Writer()
	if tee == os.Stderr {
		t.Fatal("Start did not tee the logger to a file")
	}
	stop()
	if log.Writer() != os.Stderr {
		t.Errorf("after stop the log writer is %v, want os.Stderr", log.Writer())
	}
	// The tee still holds the file: a closed file rejects the write.
	if _, err := tee.Write([]byte("")); err == nil {
		t.Error("game.log still open after stop")
	}
}
