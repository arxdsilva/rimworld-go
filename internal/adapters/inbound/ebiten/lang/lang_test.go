// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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

func TestCatalogWithLaysPluginTextOverTheBase(t *testing.T) {
	base, err := parse("pt-BR", []byte(`{"Hello": "Ola", "Count %d": "Contar %d"}`))
	if err != nil {
		t.Fatal(err)
	}
	c := base.With(map[string]string{"Hello": "Oi", "Salt-flat surveyor": "Agrimensor", "Count %d": "Contar %s", "Blank": ""})
	for src, want := range map[string]string{"Hello": "Oi", "Salt-flat surveyor": "Agrimensor", "Count %d": "Contar %d", "Blank": "Blank"} {
		if got := c.T(src); got != want {
			t.Errorf("T(%q) = %q, want %q", src, got, want)
		}
	}
	if base.T("Hello") != "Ola" || base.T("Salt-flat surveyor") != "Salt-flat surveyor" {
		t.Error("With changed the base catalog")
	}
}

func TestCodesWithListsPluginOnlyLanguages(t *testing.T) {
	got := CodesWith(t.TempDir(), []string{"pt-BR", "en", "../x"})
	if len(got) != 2 || got[0].Code != "en" || got[1].Code != "pt-BR" {
		t.Errorf("CodesWith = %v", got)
	}
}
