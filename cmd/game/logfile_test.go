package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogFileKeepsTheLastRunAsPrev(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "logs")
	for _, run := range []string{"first run\n", "second run\n"} {
		f, err := openLogFile(dir)
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
	for name, want := range map[string]string{logName: "second run\n", logPrevName: "first run\n"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Errorf("%s = %q, want %q", name, b, want)
		}
	}
}
