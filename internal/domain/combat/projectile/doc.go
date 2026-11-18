// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package projectile describes attacks for the simulation and for
// presentation: Shot is a projectile in flight (DEF-1: it walks its line
// cell by cell and can strike whatever stands in its way), View is one for
// the renderer, and Trace is a short-lived record of a shot, swing or
// impact that presentation draws as tracers, flashes and puffs and the cue
// layer voices. Traces carry no gameplay.
package projectile
