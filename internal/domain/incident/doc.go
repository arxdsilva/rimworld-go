// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package incident is the storyteller's event catalogue (SIM-025): what can
// happen, how likely, how often and where, and a pure, deterministic
// weighted pick among the events allowed right now. An event may need a
// number of threat points (Def.MinPoints), spend a share of them
// (Def.PointsPct) and keep to a season (Def.Season); one def per category may
// be the filler (Def.Fill), weighing what the others leave of a fixed total. Executing an event is
// the application's job.
package incident
