// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
)

// TestLanguageSettingSwitchesTheCatalog: the Language option (OPT-001)
// loads its catalog from assets/lang; a code that does not load is English.
func TestLanguageSettingSwitchesTheCatalog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fr.json"), []byte(`{"_meta":{"name":"Français"},"Save":"Sauver"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := shellTexts{live: &lang.Live{}, dir: dir}
	if got := s.T("Save"); got != "Save" {
		t.Fatalf("before: %q", got)
	}
	s.use("fr")
	if got := s.T("Save"); got != "Sauver" {
		t.Fatalf("fr: %q", got)
	}
	s.use("zz")
	if got := s.T("Save"); got != "Save" {
		t.Fatalf("missing catalog: %q", got)
	}
	if newTexts().dir == "" {
		t.Fatal("assets/lang not found from the test")
	}
}

// TestPluginTranslationsReachT: the UI's T returns a plugin's translation for
// the active locale, over the game's catalog (PLG-055); another locale, and
// English, are untouched, and a plugin-only language can be chosen.
func TestPluginTranslationsReachT(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fr.json"), []byte(`{"Save":"Sauver","Load":"Charger"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := shellTexts{live: &lang.Live{}, dir: dir, extra: map[string]map[string]string{
		"fr":    {"Load": "Ouvrir", "Salt-flat surveyor": "Arpenteur"},
		"pt-BR": {"Salt-flat surveyor": "Agrimensor"},
	}}
	s.use("fr")
	if s.T("Save") != "Sauver" || s.T("Load") != "Ouvrir" || s.T("Salt-flat surveyor") != "Arpenteur" {
		t.Errorf("fr: %q %q %q", s.T("Save"), s.T("Load"), s.T("Salt-flat surveyor"))
	}
	s.use("pt-BR") // no game catalog: the plugin's table alone
	if s.T("Salt-flat surveyor") != "Agrimensor" || s.T("Save") != "Save" {
		t.Errorf("pt-BR: %q %q", s.T("Salt-flat surveyor"), s.T("Save"))
	}
	s.use("en")
	if s.T("Salt-flat surveyor") != "Salt-flat surveyor" {
		t.Errorf("en: %q", s.T("Salt-flat surveyor"))
	}
	if got := s.pluginLangs(); len(got) != 2 || got[0] != "fr" || got[1] != "pt-BR" {
		t.Errorf("pluginLangs = %v", got)
	}
}
