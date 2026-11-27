// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package incident is the storyteller's event catalogue (SIM-025): what can
// happen, how likely, how often and where, and a pure, deterministic
// weighted pick among the events allowed right now. Executing an event is
// the application's job.
//
// Gates. Besides its day, biome and cooldown, an event is drawn by Pick
// only while the threat points at hand reach its MinPoints, the year allows
// its Season, it acts on the home map (Target) and it is not UnderRaid.
//
// The threat die. Book.PickThreat shows what one face of the storyteller's
// die (ThreatRolls faces) holds. The threat events, walked in ID order,
// claim Weight faces each (those in season), laid out from face
// ThreatRolls-2 round the die; the filler (Def.Fill) takes the faces left.
// An event that is not open hands its face to the filler, and a face the
// caller refuses falls back round the ring of pack, manhunters and hive. The
// rotation starts at the mad animal's face, so appending a threat never
// moves the earlier ones, but patching an early event's weight shifts every
// later face.
//
// A threat event's Weight counts die faces, except Raid's and the UnderRaid
// events', whose weights split the raid face between the big threats.
// Cooldowns are checked by the caller at fire (Def.Cooled), never in the
// layout. Book.LastFired names the event of a category that fired last.
package incident
