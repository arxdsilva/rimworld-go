// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strconv"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/wasmhost"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/modding"
)

// scriptHost is the run's scripts (PLG-060, PLG-063): the loaded plugins'
// scripts/**.star and scripts/**.wasm, plugins in load order, through one
// mux. Nil only for captures, so a capture never depends on one; any other
// run gets a mux, with no segment when no loaded plugin has a script.
func scriptHost(c runContent, capturing bool) *scriptMux {
	if capturing {
		return nil
	}
	return newScriptMux(scriptPlugins(c), wasmhost.Options{})
}

// scriptPlugins are the loaded plugins with a Starlark or Wasm script, in
// load order (core has none).
func scriptPlugins(c runContent) []modding.Plugin {
	byID := map[string]modding.Plugin{}
	for _, p := range c.found {
		byID[p.Manifest.ID] = p
	}
	var out []modding.Plugin
	for _, pl := range c.catalog.Order.Loaded {
		p, ok := byID[pl.ID]
		if !ok || pl.ID == modding.Core || len(p.Scripts)+len(p.Wasm) == 0 {
			continue
		}
		out = append(out, p)
	}
	return out
}

// withScripts gives m to the games the player starts (wizard colonies)
// and loads, restores a loaded save's mod state, reports what a save
// dropped from it, and feeds the Plugins screens (the problems found while
// playing, the Wasm counters). The dev scenes (cfg.NewSim) get none. A nil m (captures) leaves cfg as it is.
// A failed build or load leaves the mux, and the running game, as they
// were.
func withScripts(cfg uiadapter.AppConfig, m *scriptMux) uiadapter.AppConfig {
	if m == nil {
		return cfg
	}
	if nb := cfg.NewSimBuilder; nb != nil {
		cfg.NewSimBuilder = func(ng uiadapter.NewGame) (uiadapter.SimBuilder, error) {
			b, err := nb(ng)
			if err != nil || b == nil {
				return b, err
			}
			return scriptedBuilder{SimBuilder: b, m: m}, nil
		}
	}
	if load := cfg.Saves.Load; load != nil {
		cfg.Saves.Load = func(path string) (uiadapter.Simulation, ports.SaveMeta, error) {
			sim, meta, err := load(path)
			if g := gameOf(sim, err); g != nil {
				m.start(g)
				m.restore(g)
			}
			return sim, meta, err
		}
	}
	if save := cfg.Saves.Save; save != nil {
		cfg.Saves.Save = func(sim uiadapter.Simulation, meta ports.SaveMeta) error {
			err := save(sim, meta) // encodes before it returns (filesave.Queue.SaveAs)
			if g, ok := sim.(*appgame.Game); ok && g != nil {
				m.saveNotes(g)
			}
			return err
		}
	}
	cfg.Plugins.Late = m.Late
	if m.wasm != nil {
		cfg.Plugins.Stats = m.Stats
	}
	return cfg
}

// scriptedBuilder hands the built colony a fresh run of the scripts.
type scriptedBuilder struct {
	uiadapter.SimBuilder
	m *scriptMux
}

func (b scriptedBuilder) Sim() (uiadapter.Simulation, error) {
	sim, err := b.SimBuilder.Sim()
	if g := gameOf(sim, err); g != nil {
		b.m.start(g)
	}
	return sim, err
}

// gameOf is sim when it is a game and err is nil; else nil.
func gameOf(sim uiadapter.Simulation, err error) *appgame.Game {
	if g, ok := sim.(*appgame.Game); ok && g != nil && err == nil {
		return g
	}
	return nil
}

// lineOf is "line N" (empty for 0).
func lineOf(n int) string {
	if n <= 0 {
		return ""
	}
	return "line " + strconv.Itoa(n)
}
