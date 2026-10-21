package assetroot

import (
	"os"
	"path/filepath"
	"testing"
)

func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets", "metadata", "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "metadata", "audio", "audio.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFindUpFromBelow(t *testing.T) {
	t.Parallel()
	root := tree(t)
	p, ok := findUp(filepath.Join(root, "bin", "deep"), filepath.Join("assets", "metadata"), true)
	if !ok || p != filepath.Join(root, "assets", "metadata") {
		t.Fatalf("findUp = %q, %v", p, ok)
	}
	if _, ok := findUp(filepath.Join(root, "bin"), filepath.Join("assets", "metadata"), false); ok {
		t.Fatal("a directory must not match a file search")
	}
	if _, ok := findUp(filepath.Join(root, "bin"), "nope", true); ok {
		t.Fatal("found a missing path")
	}
}

func TestFindParentReturnsRoot(t *testing.T) {
	root := tree(t)
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Join(root, "bin", "deep")); err != nil {
		t.Fatal(err)
	}
	got, err := FindParent("assets", filepath.Join("metadata", "audio", "audio.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "assets"))
	if g, _ := filepath.EvalSymlinks(got); g != want {
		t.Fatalf("FindParent = %q, want %q", got, want)
	}
	if _, err := Find("definitely-not-here", true); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}
