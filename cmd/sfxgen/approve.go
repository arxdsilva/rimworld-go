package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"rimworld-go/pkg/audiomanifest"
)

const elevenLabsTool = "text_to_sound_effects"

// matchLedgerIDs lists the ledger's animal ids that match list; a pattern
// that matches nothing is an error.
func matchLedgerIDs(l *Ledger, list string) ([]string, error) {
	var out []string
	for _, id := range l.ids() {
		if matchIDs(list, id) {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%q matches no sound in the ledger", list)
	}
	sort.Strings(out)
	return out, nil
}

// approveIDs marks each id's newest take approved and lists the id in the
// sound list (volume by event, files A then B).
func (a *app) approveIDs(list string) error {
	l, err := loadLedger(a.cfg.ledger)
	if err != nil {
		return err
	}
	ids, err := matchLedgerIDs(l, list)
	if err != nil {
		return err
	}
	mpath := filepath.Join(a.cfg.assets, filepath.FromSlash(audiomanifest.Rel))
	f, err := os.Open(mpath)
	if err != nil {
		return err
	}
	m, err := audiomanifest.Decode(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", mpath, err)
	}
	for _, id := range ids {
		spec, ok := events[eventOf(id)]
		if !ok {
			return fmt.Errorf("%s: unknown event", id)
		}
		i := l.latest(id, stateApproved, statePending)
		if i < 0 {
			return fmt.Errorf("%s: no take to approve (every take was rejected)", id)
		}
		aFile, bFile := a.clipFiles(id)
		if _, err := os.Stat(aFile); err != nil {
			return fmt.Errorf("%s: %s is missing; run -process -only %s first", id, aFile, id)
		}
		stem := stemOf(id)
		files := []string{animalDir + "/" + stem + ".ogg"}
		if _, err := os.Stat(bFile); err == nil {
			files = append(files, animalDir+"/"+stem+"_b.ogg")
		}
		credits := 0
		for _, j := range l.forID(id) {
			credits += l.Takes[j].Credits // every take cost, retries included
		}
		t := &l.Takes[i]
		t.Approved = a.cfg.reviewer
		e := audiomanifest.Entry{ID: id, Bus: audiomanifest.BusEffects, Volume: spec.volume, Files: files,
			Source: audiomanifest.SourceElevenLabs, Tool: elevenLabsTool, Prompt: t.Prompt, Seconds: t.Seconds,
			Credits: credits, Date: t.Date, License: a.cfg.license, Review: a.cfg.reviewer}
		if t.backend() == backendMoss {
			e.Source, e.Tool, e.License, e.Seed = audiomanifest.SourceMOSS, mossTool, mossLicense, uint64(t.Seed)
		} else if e.License == "" {
			return fmt.Errorf("%s: an ElevenLabs take needs -license with the paid plan's terms (free-tier output is non-commercial)", id)
		}
		replaced := false
		for k := range m.Sounds {
			if m.Sounds[k].ID != id {
				continue
			}
			if !audiomanifest.Generated(m.Sounds[k].Source) && !a.cfg.replace {
				return fmt.Errorf("%s: the sound list already has a %s entry; -replace swaps it for the generated sound", id, m.Sounds[k].Source)
			}
			m.Sounds[k], replaced = e, true
		}
		if !replaced {
			m.Sounds = append(m.Sounds, e)
		}
	}
	if err := m.Validate(); err != nil {
		return err
	}
	// Written like cmd/audiogen: audiomanifest.Encode, sorted by id.
	var buf bytes.Buffer
	if err := audiomanifest.Encode(&buf, m); err != nil {
		return err
	}
	if err := os.WriteFile(mpath, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := l.save(a.cfg.ledger); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "approved %d sound(s) as %s\n", len(ids), a.cfg.reviewer)
	return nil
}

// rejectIDs marks each id's newest unapproved take rejected.
func (a *app) rejectIDs(list string) error {
	l, err := loadLedger(a.cfg.ledger)
	if err != nil {
		return err
	}
	ids, err := matchLedgerIDs(l, list)
	if err != nil {
		return err
	}
	for _, id := range ids {
		i := l.latest(id, statePending)
		if i < 0 {
			return fmt.Errorf("%s: no take waiting for review", id)
		}
		l.Takes[i].Approved = "rejected"
	}
	if err := l.save(a.cfg.ledger); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "rejected %d sound(s)\n", len(ids))
	return nil
}
