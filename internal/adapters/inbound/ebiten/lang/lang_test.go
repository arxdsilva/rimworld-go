package lang

import (
	"testing"

	"rimworld-go/pkg/msgfmt"
)

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

func TestMsgIsTheTextInEnglishAndTranslatesFormatAndArgs(t *testing.T) {
	m := msgfmt.Of("%s ate %d meals (%.1f%%).", "Ann", 3, 4.25)
	text := m.String()
	if got := Msg(English{}, m, text); got != "Ann ate 3 meals (4.2%)." || got != text {
		t.Fatalf("English Msg = %q, want %q", got, text)
	}
	if got := Msg(fake{"x": "y"}, msgfmt.Text{}, "posted text"); got != "posted text" {
		t.Fatalf("a view without Msg must show its Text, got %q", got)
	}
	fr := fake{"%s ate %s meals (%s%%).": "%s a mangé %s repas (%s%%).", "boar": "sanglier", "%s tamed a %s.": "%[2]s apprivoisé par %[1]s."}
	if got := Msg(fr, m, text); got != "Ann a mangé 3 repas (4.2%)." {
		t.Fatalf("translated Msg = %q", got)
	}
	tamed := msgfmt.Of("%s tamed a %s.", "Ann", "boar")
	if got := Msg(fr, tamed, tamed.String()); got != "sanglier apprivoisé par Ann." {
		t.Fatalf("args are translated by exact key: %q", got)
	}
}
