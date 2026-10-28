---
name: rimworld-go-creatures
description: Add, change or remove an animal or creature in rimworld-go with the same full setup every time — roster row, simulation numbers, art from the animal rig, the four generated sounds (call, attack, hurt, death) and every hook that makes it live in play. Use whenever a species, creature, monster, insect, bird or pet is created, renamed, redesigned or retired.
---

# Creatures: the one setup every species gets

A creature is only "done" when **every** row of the checklist below holds. No creature ships silent, invisible, or half-wired. The tests listed at the end fail when a step is missing — never weaken them to pass.

Canonical sources (read by section, never paste whole files):

| What | Where |
|---|---|
| Roster (sim, binding) | `internal/domain/animal/animal.go` — `Roster` rows, `buildDef` formulas |
| Roster (art, mirror) | `assets/source/blender/artlib/families/animal_roster.py` |
| Art rules | ART spec §11 / §11.1 (roster, originals rule, sprite set), §16.2 (blood), §6.1 (shadows) |
| Sound rules | `.cursor/handoffs/SND-001-audio-runtime.md` (addendum: animal voices), `SND-003-animal-voices.md` once written; plan of record for ids, fallback, spending |
| Wildlife behaviour | `.cursor/handoffs/SIM-019-wildlife.md` |

## Hard rules

1. **Originals only.** Never reproduce RimWorld-invented creatures (names, look, sounds or balance). Real animals are fine; fictional roles get our own design and name (see ART §11.1). Prompts and blurbs never name RimWorld creatures.
2. **Append-only roster.** New species go before `numSpecies`; never reorder or delete a value (saves store the uint8). Retire a species by aliasing its key in `ParseSpecies`, not by removing its slot.
3. **One key everywhere.** The snake_case roster key is the sim name, the art key (`animal/<key>_…`), the sound key (`animal.<key>.<event>`), the icon (`ui_icon/<key>`), the corpse (`item/corpse_<key>_v0..v2`) and the blurb key. Go roster and Python roster list the same keys in the same order.
4. **Our own numbers.** Stats come from `buildDef` (size × behaviour classes). Add a per-species override only when the brief demands it, documented in the override map.
5. **Every creature has its four voices** — `call`, `attack`, `hurt`, `die` — generated, reviewed and approved (below). Until approved, the voice-group placeholder plays; that is a temporary state recorded in the handoff, never the end state.
6. **Spending is user-approved.** ElevenLabs calls cost credits: dry-run first, the user approves each batch, every take is logged in the ledger, the reserve is never crossed, at most one retry per sound. Never print, log or commit the API key.

## Checklist (do all, in this order)

### 1. Roster and simulation
- [ ] `animal.go`: append the `Roster` row — key, archetype, size (+ trim), behaviour, habitat, natural weapon (`AttackKind`), herd min/max, speed class, flags (dimorphic, explosive, small-prey; hive via behaviour).
- [ ] Update the roster count test (`TestRosterHas61` → the new total) deliberately, in the same change.
- [ ] Behaviour needs nothing else if it fits an existing class; a genuinely new behaviour gets its own SIM task (determinism skill) first.
- [ ] Spawning: habitat/rarity are enough for wildlife; threats (packs, swarms, events) only via `wildlife.go`/`hive.go` rules.
- [ ] Blood kind: hive behaviour → ichor; any other new fluid needs a `stain.Kind` and renderer colour (SIM-020 / ART §16.2).

### 2. Art (Blender animal rig)
- [ ] `animal_roster.py`: the mirrored row with a **features** brief (silhouette, proportions, palette, signature traits) and the `attack` field matching Go.
- [ ] Builder spec in `families/animals/<archetype>.py` (`quadruped.py`, `bird.py`, `reptile.py`, `insect.py`); new materials additively (`materials.yaml` or a `materials_*.yaml` extension).
- [ ] Render with a private temp dir: `TMPDIR=<scratch>/tmp ANIMALS=<key> make art FAMILIES=animal`, then `FAMILIES=overlay` for the icon. Full set per rendered facing (south, east, north; west is mirrored by the renderer): standing, `walk01..08`, attack set `<attack>1..4`, `graze1..2`, `flinch1..2`; corpses `v0..v2`; `_f` set when dimorphic.
- [ ] Size is the AREA the body covers (PLAY-022, ART §11.1): the size class sets the length from `SIZE_LENGTH`, and `validate_assets.py` checks each class median against `SIZE_AREA`. The rig writes `ppt` (the facing's density) and `gait` (ground per walk cycle, so the feet stay planted) into every sprite's metadata. Don't hand-tune either.
- [ ] `make normals` and `make art-check` (0 errors); review a contact sheet + 64 px silhouette + in-game `-scenario zoo` capture by eye.

### 3. Sounds (generated, mandatory)
- [ ] `assets/source/audio/animal_prompts.json`: four rows `animal.<key>.call|attack|hurt|die` — our own words describing this creature (size shapes the voice; call = its characteristic vocalisation, attack = aggressive, hurt = short pained, die = long fading), lengths per the SND rules (call 1.0–2.5 s, attack 0.8, hurt 0.6, die 1.0–2.5), suffix from `defaults`.
- [ ] `render/cue/animal_sounds.go` `VoiceGroups`: map the key to its fallback group (`small_mammal`, `large_mammal`, `canid`, `felid`, `bovid`, `bird_small`, `bird_large`, `reptile`, `insect`).
- [ ] Generate with `cmd/sfxgen`: `-dry-run -only <key>` (show the credits) → **user approves** → generate → `make animal-audio` (ffmpeg + oggenc: trim, loudness, OGG, pitch variant) → `sfxgen -review` page → **user listens and approves** → `sfxgen -approve <ids>` (writes the ledger, `audio.json` with license and `reviewed_by`).
- [ ] `make audio-check` passes (decodes, 48 kHz, budget, roster coverage).
- [ ] License line on every ElevenLabs entry (free tier: non-commercial, credit ElevenLabs) — or the paid licence if the account changed; `THIRD_PARTY_NOTICES.md` kept true.

### 4. Presentation hooks
- [ ] `animal_blurbs.go`: one original line (≤ 70 chars).
- [ ] Nothing else is per-species in the renderer (frames by `Attack`, `_f` by `Female`, markers by `Size`, corpses by `ItemView.Animal`, shadows and blood by figure) — if you find yourself adding a species switch, extend the roster data instead.

### 5. Close out
- [ ] Full gates (`.cursor/PARALLEL_WORK.md` §5): build, fmt, vet, lint, `-race`, determinism ×3, `make art-check`, `make audio-check`, benchmarks.
- [ ] Handoff note (species, art review, sounds generated/approved with credits spent) and checklist boxes (vanilla Animals / Wildlife, UI/UX animals tab) — `[x]`/`[-]`/`[!]` honestly.

## Tests that enforce this (must stay green; never weaken)
| Test | Catches |
|---|---|
| `TestRosterHas61` (count), `TestEveryDefComplete`, `TestSpeciesRules` (`internal/domain/animal`) | roster row missing or incomplete numbers |
| `TestEverySpeciesHasArt` (`ebiten/art_test.go`) + `validate_assets.py` roster check | missing sprites, frames, corpses, `_f` sets, icon |
| `TestBlurbsForAll` | missing blurb |
| `TestEveryRosterVoiceHasAGroup` | missing voice group |
| `TestPromptsCoverRoster` (`cmd/sfxgen`) | missing or forbidden-named prompts |
| `make audio-check` roster coverage | a roster key whose four events don't resolve to a playable sound |

## Parallel work
A batch of new creatures is multi-area (domain, art, audio, renderer, docs): follow `.cursor/PARALLEL_WORK.md` — the lead writes the roster rows in Go **and** Python first (the contract), then art, sound and sim workers run in parallel; paid generation stays lead-gated and user-approved.
