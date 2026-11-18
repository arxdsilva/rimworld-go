// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strconv"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/inbound/ebiten/plugins"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/starlarkhost"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/modding"
)

// scriptHost is the run's script host (PLG-060): the loaded plugins'
// scripts/**.star, plugins in load order. Nil when no loaded plugin has a
// script, and always for captures, so a capture never depends on one.
func scriptHost(c runContent, capturing bool) *starlarkhost.Host {
	if capturing {
		return nil
	}
	ps := scriptPlugins(c)
	if len(ps) == 0 {
		return nil
	}
	return starlarkhost.New(ps)
}

// scriptPlugins are the loaded plugins' scripts in load order (core has
// none).
func scriptPlugins(c runContent) []starlarkhost.Plugin {
	byID := map[string]modding.Plugin{}
	for _, p := range c.found {
		byID[p.Manifest.ID] = p
	}
	var out []starlarkhost.Plugin
	for _, pl := range c.catalog.Order.Loaded {
		p, ok := byID[pl.ID]
		if !ok || pl.ID == modding.Core || len(p.Scripts) == 0 {
			continue
		}
		sp := starlarkhost.Plugin{ID: pl.ID}
		for _, b := range p.Scripts {
			sp.Files = append(sp.Files, starlarkhost.File{Path: b.Path, Src: b.Data})
		}
		out = append(out, sp)
	}
	return out
}

// withScripts gives h to the games the player starts (wizard colonies)
// and loads, and shows the scripts it turned off on the Plugins screens.
// The dev scenes (cfg.NewSim) get none. A nil h leaves cfg as it is.
func withScripts(cfg uiadapter.AppConfig, h *starlarkhost.Host) uiadapter.AppConfig {
	if h == nil {
		return cfg
	}
	if nb := cfg.NewSimBuilder; nb != nil {
		cfg.NewSimBuilder = func(ng uiadapter.NewGame) (uiadapter.SimBuilder, error) {
			b, err := nb(ng)
			if err != nil || b == nil {
				return b, err
			}
			return scriptedBuilder{SimBuilder: b, h: h}, nil
		}
	}
	if load := cfg.Saves.Load; load != nil {
		cfg.Saves.Load = func(path string) (uiadapter.Simulation, ports.SaveMeta, error) {
			sim, meta, err := load(path)
			useScripts(sim, h)
			return sim, meta, err
		}
	}
	cfg.Plugins.Late = func() []plugins.Problem { return scriptProblems(h) }
	return cfg
}

// scriptedBuilder hands the built colony the script host.
type scriptedBuilder struct {
	uiadapter.SimBuilder
	h *starlarkhost.Host
}

func (b scriptedBuilder) Sim() (uiadapter.Simulation, error) {
	sim, err := b.SimBuilder.Sim()
	useScripts(sim, b.h)
	return sim, err
}

// useScripts sets h on sim when it is a game.
func useScripts(sim uiadapter.Simulation, h *starlarkhost.Host) {
	if g, ok := sim.(*appgame.Game); ok && g != nil {
		appgame.UseScripts(g, h)
	}
}

// scriptProblems are h's turned-off scripts as Plugins screen problems.
func scriptProblems(h *starlarkhost.Host) []plugins.Problem {
	var out []plugins.Problem
	for _, p := range h.Problems() {
		out = append(out, plugins.Problem{Plugin: p.Plugin, File: p.File, Field: lineOf(p.Line),
			Msg: "script turned off for this run: " + p.Msg})
	}
	return out
}

// lineOf is "line N" (empty for 0).
func lineOf(n int) string {
	if n <= 0 {
		return ""
	}
	return "line " + strconv.Itoa(n)
}
