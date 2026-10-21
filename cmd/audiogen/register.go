package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"rimworld-go/pkg/audiomanifest"
)

// promptsRel is the ElevenLabs prompt list, relative to the assets root.
const promptsRel = "source/audio/prompts.json"

// prompt is one generated sound in prompts.json.
type prompt struct {
	ID      string  `json:"id"`
	File    string  `json:"file"` // under generated/audio
	Tool    string  `json:"tool"`
	Seconds float64 `json:"seconds"`
	Loop    bool    `json:"loop"`
	Volume  float64 `json:"volume"`
	Prompt  string  `json:"prompt"`
}

func readPrompts(assets string) ([]prompt, error) {
	b, err := os.ReadFile(filepath.Join(assets, filepath.FromSlash(promptsRel)))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Sounds []prompt `json:"sounds"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", promptsRel, err)
	}
	return doc.Sounds, nil
}

func promptFor(assets, file string) (prompt, error) {
	ps, err := readPrompts(assets)
	if err != nil {
		return prompt{}, err
	}
	for _, p := range ps {
		if p.File == file {
			return p, nil
		}
	}
	return prompt{}, fmt.Errorf("%s is not in %s", file, promptsRel)
}

// register adds an imported, approved file to the sound list under its
// prompt's id (a second file of an id becomes another variant).
func register(assets, file string, credits int, license, reviewer string) (audiomanifest.Entry, error) {
	p, err := promptFor(assets, file)
	if err != nil {
		return audiomanifest.Entry{}, err
	}
	if _, err := os.Stat(filepath.Join(assets, filepath.FromSlash(audiomanifest.Dir), filepath.FromSlash(file))); err != nil {
		return audiomanifest.Entry{}, fmt.Errorf("import %s first: %w", file, err)
	}
	m, err := readManifest(assets)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return audiomanifest.Entry{}, err
	}
	e, i := audiomanifest.Entry{}, -1
	for k, x := range m.Sounds {
		if x.ID == p.ID {
			e, i = x, k
		}
	}
	if i >= 0 && e.Source != audiomanifest.SourceElevenLabs {
		return audiomanifest.Entry{}, fmt.Errorf("%s is already a %s sound", p.ID, e.Source)
	}
	e.ID, e.Bus, e.Loop, e.Volume = p.ID, audiomanifest.BusOf(p.ID), p.Loop, p.Volume
	e.Source, e.Tool, e.Prompt, e.Seconds = audiomanifest.SourceElevenLabs, p.Tool, p.Prompt, p.Seconds
	e.Credits += credits
	e.Date = time.Now().Format("2006-01-02")
	e.License, e.Review = license, reviewer
	if !contains(e.Files, file) {
		e.Files = append(e.Files, file)
	}
	if i >= 0 {
		m.Sounds[i] = e
	} else {
		m.Sounds = append(m.Sounds, e)
	}
	if err := m.Validate(); err != nil {
		return audiomanifest.Entry{}, err
	}
	return e, writeManifest(assets, m)
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
