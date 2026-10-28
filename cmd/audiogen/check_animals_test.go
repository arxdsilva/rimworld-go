package main

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/pkg/audiomanifest"
)

func animalManifest(ids ...string) audiomanifest.Manifest {
	var m audiomanifest.Manifest
	for _, id := range ids {
		m.Sounds = append(m.Sounds, audiomanifest.Entry{ID: id, Bus: audiomanifest.BusEffects, Volume: 1,
			Files: []string{"sfx/animals/x.wav"}, Source: audiomanifest.SourceProcedural, License: "x"})
	}
	return m
}

func allGroups() map[string]string {
	g := map[string]string{}
	for _, s := range animal.All() {
		g[s.String()] = "canid"
	}
	return g
}

func TestAnimalCoverageFallbackChain(t *testing.T) {
	t.Parallel()
	// generic alone covers everything
	if err := checkAnimalCoverage(animalManifest("animal.generic.call", "animal.generic.attack", "animal.generic.hurt", "animal.generic.die"), allGroups()); err != nil {
		t.Fatal(err)
	}
	// group sounds cover the species whose group has them, generic the rest
	m := animalManifest("animal.canid.call", "animal.generic.call", "animal.generic.attack", "animal.generic.hurt", "animal.generic.die")
	if err := checkAnimalCoverage(m, allGroups()); err != nil {
		t.Fatal(err)
	}
	// a hole: no generic die, no species die
	err := checkAnimalCoverage(animalManifest("animal.generic.call", "animal.generic.attack", "animal.generic.hurt"), allGroups())
	if err == nil || !strings.Contains(err.Error(), ".die resolves to no sound") {
		t.Fatalf("err = %v", err)
	}
	// a stray group entry
	g := allGroups()
	g["dragon"] = "reptile"
	if err := checkAnimalCoverage(animalManifest("animal.generic.call"), g); err == nil {
		t.Fatal("a non-roster key in voiceGroups was accepted")
	}
}

func TestParseVoiceGroupsAndRealTable(t *testing.T) {
	t.Parallel()
	if _, err := parseVoiceGroups("package x"); err == nil {
		t.Fatal("missing table accepted")
	}
	if _, err := parseVoiceGroups("var VoiceGroups = map[string]string{\n}\n"); err == nil {
		t.Fatal("empty table accepted")
	}
	got, err := parseVoiceGroups("var VoiceGroups = map[string]string{\n\t\"cow\": \"bovid\", \"cat\": \"felid\",\n}\n")
	if err != nil || got["cow"] != "bovid" || got["cat"] != "felid" || len(got) != 2 {
		t.Fatalf("got %v, %v", got, err)
	}
	// the real table: parsed from the renderer source, it covers the roster
	if err := checkAnimals("../../assets", mustManifest(t)); err != nil {
		t.Fatal(err)
	}
}

func mustManifest(t *testing.T) audiomanifest.Manifest {
	t.Helper()
	m, err := readManifest("../../assets")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCheckAnimalsSkippedWithoutRendererSource(t *testing.T) {
	t.Parallel()
	if err := checkAnimals(t.TempDir()+"/assets", audiomanifest.Manifest{}); err != nil {
		t.Fatal(err)
	}
}
