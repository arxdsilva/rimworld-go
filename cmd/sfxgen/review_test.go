package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/pkg/audiomanifest"
)

func fakeClips(t *testing.T, e *env, files ...string) {
	t.Helper()
	dir := filepath.Join(e.assets, "generated/audio/sfx/animals")
	os.MkdirAll(dir, 0o755)
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("OggS fake "+f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReviewPageListsClips(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.lufs = func(string) (float64, error) { return -20.46, nil }
	e.writeLedger(
		Take{ID: "animal.cow.call", Take: 1, Seconds: 2, Credits: 20, Prompt: "a <deep> cow moo"},
		Take{ID: "animal.cow.die", Take: 1, Seconds: 1.8, Credits: 18, Prompt: "cow dies", Approved: "arxdsilva"},
		Take{ID: "animal.pig.call", Take: 1, Seconds: 1, Credits: 10, Prompt: "unprocessed pig"},
	)
	fakeClips(t, e, "cow_call.ogg", "cow_call_b.ogg", "cow_die.ogg")
	page := filepath.Join(e.dir, "out", "review.html")
	if _, _, err := e.cli("-review", "-o", page); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(page)
	html := string(b)
	contains(t, html, "animal.cow.call (take 1)", "animal.cow.die", `<audio controls`,
		"../assets/generated/audio/sfx/animals/cow_call.ogg", "../assets/generated/audio/sfx/animals/cow_call_b.ogg",
		"a &lt;deep&gt; cow moo", "approved by arxdsilva", "pending", "-20.5 / -20.5", "-20.5 / -", "<td>20</td>")
	if strings.Contains(html, "animal.pig.call") {
		t.Fatal("an unprocessed clip is listed")
	}
	if strings.Count(html, "<audio") != 3 {
		t.Fatalf("%d players, want 3 (cow call A+B, cow die A)", strings.Count(html, "<audio"))
	}
}

func TestReviewToStdoutAndOnly(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.writeLedger(Take{ID: "animal.cow.call", Take: 1, Prompt: "cow"}, Take{ID: "animal.cow.hurt", Take: 1, Prompt: "hurt"})
	fakeClips(t, e, "cow_call.ogg", "cow_hurt.ogg")
	out, _, err := e.cli("-review", "-only", "animal.cow.hurt")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, out, "animal.cow.hurt", "file://")
	if strings.Contains(out, "animal.cow.call") {
		t.Fatal("-only ignored")
	}
}

func TestApproveWritesManifestSorted(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	mk := func(id, src string) audiomanifest.Entry {
		return audiomanifest.Entry{ID: id, Bus: audiomanifest.BusEffects, Volume: 0.5, Files: []string{"sfx/x.wav"}, Source: src, License: "CC0"}
	}
	existing := audiomanifest.Manifest{Sounds: []audiomanifest.Entry{mk("amb.day", audiomanifest.SourceElevenLabs), mk("animal.canid.call", audiomanifest.SourceProcedural), mk("sfx.chop", audiomanifest.SourceProcedural), mk("ui.click", audiomanifest.SourceProcedural)}}
	mpath := filepath.Join(e.assets, filepath.FromSlash(audiomanifest.Rel))
	os.MkdirAll(filepath.Dir(mpath), 0o755)
	writeManifestFile(t, mpath, existing)
	before, _ := os.ReadFile(mpath)

	e.writeLedger(
		Take{ID: "animal.cow.hurt", Take: 1, Seconds: 0.6, Credits: 6, Prompt: "hurt1", Date: "2026-10-22", Approved: "rejected"},
		Take{ID: "animal.cow.hurt", Take: 2, Seconds: 0.6, Credits: 6, Prompt: "hurt2", Date: "2026-10-23"},
		Take{ID: "animal.cow.call", Take: 1, Seconds: 2, Credits: 20, Prompt: "call1", Date: "2026-10-22"},
		Take{ID: "animal.cow.die", Take: 1, Seconds: 1.8, Credits: 18, Prompt: "die1", Date: "2026-10-22"},
		Take{ID: "animal.canid.call", Take: 1, Seconds: 1.5, Credits: 15, Prompt: "canid", Date: "2026-10-22"},
	)
	fakeClips(t, e, "cow_call.ogg", "cow_call_b.ogg", "cow_hurt.ogg", "cow_hurt_b.ogg", "cow_die.ogg", "canid_call.ogg")
	if _, _, err := e.cli("-approve", "animal.cow.*"); err != nil {
		t.Fatal(err)
	}
	m := readManifestFile(t, mpath)
	var ids []string
	for _, s := range m.Sounds {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, " ") != "amb.day animal.canid.call animal.cow.call animal.cow.die animal.cow.hurt sfx.chop ui.click" {
		t.Fatalf("ids = %v", ids)
	}
	for i, e0 := range existing.Sounds { // untouched entries
		if got, _ := m.Lookup(e0.ID); got.Source != e0.Source || got.Volume != 0.5 || got.Files[0] != "sfx/x.wav" || i < 0 {
			t.Fatalf("%s changed: %+v", e0.ID, got)
		}
	}
	call, _ := m.Lookup("animal.cow.call")
	want := audiomanifest.Entry{ID: "animal.cow.call", Bus: "effects", Volume: 0.7, Files: []string{"sfx/animals/cow_call.ogg", "sfx/animals/cow_call_b.ogg"},
		Source: "elevenlabs", Tool: "text_to_sound_effects", Prompt: "call1", Seconds: 2, Credits: 20, Date: "2026-10-22",
		License: "ElevenLabs free tier: non-commercial use, credit ElevenLabs", Review: "arxdsilva"}
	if call.ID != want.ID || call.Bus != want.Bus || call.Volume != want.Volume || strings.Join(call.Files, ",") != strings.Join(want.Files, ",") ||
		call.Source != want.Source || call.Tool != want.Tool || call.Prompt != want.Prompt || call.Seconds != want.Seconds ||
		call.Credits != want.Credits || call.Date != want.Date || call.License != want.License || call.Review != want.Review {
		t.Fatalf("call = %+v\nwant   %+v", call, want)
	}
	hurt, _ := m.Lookup("animal.cow.hurt")
	if hurt.Volume != 0.85 || hurt.Prompt != "hurt2" || hurt.Credits != 12 || hurt.Date != "2026-10-23" {
		t.Fatalf("hurt = %+v (the retry take, both takes' credits)", hurt)
	}
	die, _ := m.Lookup("animal.cow.die")
	if die.Volume != 1.0 || len(die.Files) != 1 {
		t.Fatalf("die = %+v (no variant file)", die)
	}
	l := e.readLedger()
	if l.Takes[1].Approved != "arxdsilva" || l.Takes[0].Approved != "rejected" || l.Takes[2].Approved != "arxdsilva" {
		t.Fatalf("ledger = %+v", l.Takes)
	}
	// idempotent
	after, _ := os.ReadFile(mpath)
	if _, _, err := e.cli("-approve", "animal.cow.*"); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(mpath)
	if string(after) != string(again) || string(before) == string(after) {
		t.Fatal("approve is not idempotent")
	}
	// a placeholder is not replaced unless asked
	if _, _, err := e.cli("-approve", "animal.canid.call"); err == nil {
		t.Fatal("a procedural placeholder was replaced silently")
	}
	if _, _, err := e.cli("-approve", "animal.canid.call", "-replace"); err != nil {
		t.Fatal(err)
	}
	if c, _ := readManifestFile(t, mpath).Lookup("animal.canid.call"); c.Source != "elevenlabs" {
		t.Fatal("-replace did not replace")
	}
}

func TestApproveRefusals(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	mpath := filepath.Join(e.assets, filepath.FromSlash(audiomanifest.Rel))
	os.MkdirAll(filepath.Dir(mpath), 0o755)
	writeManifestFile(t, mpath, audiomanifest.Manifest{})
	e.writeLedger(Take{ID: "animal.cow.call", Take: 1, Approved: "rejected"}, Take{ID: "animal.cow.hurt", Take: 1})
	if _, _, err := e.cli("-approve", "animal.cow.call"); err == nil {
		t.Fatal("approved a rejected-only id")
	}
	if _, _, err := e.cli("-approve", "animal.cow.hurt"); err == nil || !strings.Contains(err.Error(), "-process") {
		t.Fatalf("approved an unprocessed clip: %v", err)
	}
	e2 := newEnv(t)
	e2.writeLedger(Take{ID: "animal.cow.hurt", Take: 1})
	if _, _, err := e2.cli("-approve", "animal.cow.hurt"); err == nil {
		t.Fatal("approved with no sound list")
	}
}

func TestRejectMarksLedger(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.writeLedger(Take{ID: "animal.cow.call", Take: 1}, Take{ID: "animal.cow.hurt", Take: 1, Approved: "arxdsilva"})
	if _, _, err := e.cli("-reject", "animal.cow.call"); err != nil {
		t.Fatal(err)
	}
	if got := e.readLedger().Takes[0].Approved; got != "rejected" {
		t.Fatalf("approved = %q", got)
	}
	if _, _, err := e.cli("-reject", "animal.cow.call"); err == nil {
		t.Fatal("rejected twice")
	}
	if _, _, err := e.cli("-reject", "animal.cow.hurt"); err == nil {
		t.Fatal("rejected an approved take")
	}
}

func writeManifestFile(t *testing.T, path string, m audiomanifest.Manifest) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := audiomanifest.Encode(f, m); err != nil {
		t.Fatal(err)
	}
}

func readManifestFile(t *testing.T, path string) audiomanifest.Manifest {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := audiomanifest.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
