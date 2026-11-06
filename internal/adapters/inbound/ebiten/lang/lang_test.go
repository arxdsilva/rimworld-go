package lang

import "testing"

func TestEnglishIsTheSourceText(t *testing.T) {
	var l *Live
	if l.T("Drop") != "Drop" || l.Tf("%d kg", 3) != "3 kg" || Tf(English{}, "%s!", "Go") != "Go!" {
		t.Fatal("English changed the text")
	}
	l = &Live{}
	l.Set(fake{"Drop": "Lâcher", "%d kg": "%d kilos"})
	if l.T("Drop") != "Lâcher" || l.Tf("%d kg", 3) != "3 kilos" || l.T("Eat") != "Eat" {
		t.Fatalf("Live ignores its catalog: %q %q", l.T("Drop"), l.Tf("%d kg", 3))
	}
}

type fake map[string]string

func (f fake) T(s string) string {
	if v, ok := f[s]; ok {
		return v
	}
	return s
}
