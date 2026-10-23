package scenariofile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSaveLoadListDeleteRoundTrip(t *testing.T) {
	t.Parallel()
	s := Store{Dir: filepath.Join(t.TempDir(), "nested", "scenarios")} // Save creates the folder
	if err := s.Save("zeta", []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("alpha", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("alpha", []byte("two")); err != nil { // replaces
		t.Fatal(err)
	}
	got, err := s.Load("alpha")
	if err != nil || string(got) != "two" {
		t.Fatalf("load = %q, %v; want the replacement", got, err)
	}
	files, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
		if f.Modified.IsZero() {
			t.Errorf("%s has no modified time", f.Name)
		}
	}
	if !reflect.DeepEqual(names, []string{"alpha", "zeta"}) {
		t.Fatalf("list = %v, want sorted names", names)
	}
	if err := s.Delete("alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("alpha"); err == nil {
		t.Fatal("a deleted scenario still loads")
	}
	if err := s.Delete("alpha"); err != nil {
		t.Fatalf("deleting a missing file = %v, want nil", err)
	}
}

func TestSaveIsAtomicAndLeavesNoTempFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := Store{Dir: dir}
	for i := 0; i < 3; i++ {
		if err := s.Save("one", []byte(strings.Repeat("x", 100*(i+1)))); err != nil {
			t.Fatal(err)
		}
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 || ents[0].Name() != "one.json" {
		t.Fatalf("folder = %v, want only one.json", ents)
	}
}

func TestListIgnoresForeignFilesAndMissingFolder(t *testing.T) {
	t.Parallel()
	missing := Store{Dir: filepath.Join(t.TempDir(), "none")}
	if files, err := missing.List(); err != nil || len(files) != 0 {
		t.Fatalf("missing folder = %v, %v; want empty", files, err)
	}
	dir := t.TempDir()
	for _, n := range []string{"keep.json", "notes.txt", "a b.json", ".hidden.json", "x.json.tmp"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := Store{Dir: dir}.List()
	if err != nil || len(files) != 1 || files[0].Name != "keep" {
		t.Fatalf("list = %v, %v; want only keep", files, err)
	}
}

func TestListReportsAnUnreadableFolder(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Dir: file}).List(); err == nil {
		t.Fatal("a file where the folder should be must be an error")
	}
	if err := (Store{Dir: filepath.Join(file, "sub")}).Save("x", nil); err == nil {
		t.Fatal("saving under a file must fail")
	}
}

func TestNamesCannotEscapeTheFolder(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	for _, bad := range []string{"", "../evil", "a/b", `a\b`, "..", "a.b", strings.Repeat("a", 65), "sp ace"} {
		if ValidName(bad) {
			t.Errorf("ValidName(%q) = true", bad)
		}
		if err := s.Save(bad, []byte("x")); err == nil {
			t.Errorf("Save(%q) succeeded", bad)
		}
		if _, err := s.Load(bad); err == nil {
			t.Errorf("Load(%q) succeeded", bad)
		}
		if err := s.Delete(bad); err == nil {
			t.Errorf("Delete(%q) succeeded", bad)
		}
	}
	for _, ok := range []string{"a", "Broken-Hull_2", strings.Repeat("a", 64)} {
		if !ValidName(ok) {
			t.Errorf("ValidName(%q) = false", ok)
		}
	}
}

func TestDefaultIsUnderTheUserConfigDir(t *testing.T) {
	t.Parallel()
	s, err := Default()
	if err != nil {
		t.Skip("no user config dir:", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(s.Dir), "rimworld-go/scenarios") {
		t.Fatalf("Default dir = %s", s.Dir)
	}
}
