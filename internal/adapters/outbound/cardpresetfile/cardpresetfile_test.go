package cardpresetfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "dossier")}
	if l, err := s.List(); err != nil || len(l) != 0 {
		t.Fatalf("missing folder: %v %v", l, err)
	}
	if err := s.Save("my-card_1", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("alpha", []byte("x")); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(s.Dir, "stray.txt"), []byte("x"), 0o600)
	_ = os.WriteFile(filepath.Join(s.Dir, "bad name.json"), []byte("x"), 0o600)
	l, _ := s.List()
	if strings.Join(l, ",") != "alpha,my-card_1" {
		t.Fatalf("list = %v", l)
	}
	b, err := s.Load("my-card_1")
	if err != nil || string(b) != `{"a":1}` {
		t.Fatalf("load = %q %v", b, err)
	}
	if err := s.Delete("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("alpha"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if l, _ = s.List(); len(l) != 1 {
		t.Fatalf("after delete: %v", l)
	}
}

func TestRefusesNamesThatEscape(t *testing.T) {
	root := t.TempDir()
	s := Store{Dir: filepath.Join(root, "dossier")}
	for _, n := range []string{"", "..", ".", "../x", "a/b", `a\b`, "a.b", "/etc/passwd", "a b", strings.Repeat("a", MaxName+1), "x\x00y"} {
		if err := s.Save(n, []byte("x")); !errors.Is(err, ErrName) {
			t.Errorf("Save(%q) = %v", n, err)
		}
		if _, err := s.Load(n); !errors.Is(err, ErrName) {
			t.Errorf("Load(%q) = %v", n, err)
		}
		if err := s.Delete(n); !errors.Is(err, ErrName) {
			t.Errorf("Delete(%q) = %v", n, err)
		}
	}
	if es, _ := os.ReadDir(root); len(es) != 0 {
		t.Fatalf("a refused save touched the disk: %v", es)
	}
}

func TestSizeCap(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Save("big", make([]byte, MaxBytes+1)); err == nil {
		t.Fatal("oversized save accepted")
	}
	_ = os.WriteFile(filepath.Join(s.Dir, "huge.json"), make([]byte, MaxBytes+10), 0o600)
	if _, err := s.Load("huge"); err == nil {
		t.Fatal("oversized load accepted")
	}
}
