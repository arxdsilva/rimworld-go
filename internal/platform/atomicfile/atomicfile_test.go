package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestWriteCreatesTheFileAndItsFolder(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "a", "b", "file.txt")
	if err := WriteBytes(path, 0o644, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "new" {
		t.Fatalf("read %q, %v", got, err)
	}
	if left := names(t, filepath.Dir(path)); len(left) != 1 {
		t.Errorf("folder holds %v, want only the file", left)
	}
}

func TestWriteReplacesTheOldContents(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "file.txt")
	for _, want := range []string{"first, the longer one", "second"} {
		if err := WriteBytes(path, 0o644, []byte(want)); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(path); string(got) != want {
			t.Errorf("read %q, want %q", got, want)
		}
	}
}

func TestAFailedWriteLeavesTheOldFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := WriteBytes(path, 0o644, []byte("old")); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("disk full")
	err := Write(path, 0o644, func(w io.Writer) error {
		_, _ = w.Write([]byte("half of the n"))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error %v, want %v", err, boom)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("read %q after a failed write", got)
	}
	if left := names(t, filepath.Dir(path)); len(left) != 1 {
		t.Errorf("folder holds %v, want no temporary file", left)
	}
}

func TestAFailedRenameLeavesNoTemporaryFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "taken")
	inside := filepath.Join(path, "child")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteBytes(path, 0o644, []byte("x")); err == nil {
		t.Fatal("replacing a folder with a file must fail")
	}
	if left := names(t, dir); len(left) != 1 || left[0] != "taken" {
		t.Errorf("folder holds %v", left)
	}
}

func TestWriteSetsTheMode(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix file modes")
	}
	dir := t.TempDir()
	for name, perm := range map[string]os.FileMode{"private": 0o600, "shared": 0o644} {
		path := filepath.Join(dir, name)
		if err := WriteBytes(path, perm, nil); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != perm {
			t.Errorf("%s: mode %o, want %o", name, info.Mode().Perm(), perm)
		}
	}
}
