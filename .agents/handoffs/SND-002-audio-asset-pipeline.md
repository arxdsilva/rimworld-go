# Task Handoff

Task ID: SND-002
Status: effects complete; ElevenLabs music and ambience pending (needs the server connected)

## Summary

All 26 sound effects are generated from code: UI, work, doors, hits, gunshot, thunder and the campfire crackle loop. They take 3.4 MB of the 20 MB budget. The ElevenLabs pipeline is ready: MCP config, prompts in spending order within a 10,000-credit budget, a credit ledger, and an import script.

## Files Changed

- `pkg/sfxsynth` (new): synthesis, seeded dither, WAV read/write, loop crossfade
- `pkg/audiomanifest` (new): the sound list format and its validation
- `cmd/audiogen` (new):
  - `recipes.go`;
  - generation, `-check`, `-loopfix`, `-looping` and `-register`.
- `assets/generated/audio/{ui,sfx,ambience}/*.wav`, `assets/metadata/audio/audio.json` (generated)
- `assets/source/audio/prompts.json`, `assets/source/audio/elevenlabs/ledger.json`
- `scripts/audio_import.sh`
- `.mcp.json`, `.cursor/mcp.json`: the `elevenlabs` server, with `ELEVEN_LABS_API_KEY` from the user's shell mapped to `ELEVENLABS_API_KEY`
- `.gitignore`: raw takes
- `Makefile`: `audio`, `audio-check`

## Invariants / Decisions

- Effects are byte-identical across runs and machines (`TestGenerateIsDeterministicAndKeepsOtherSources`).
- Regenerating keeps every non-procedural entry in the sound list.
- ElevenLabs spending order:
  1. calm music
  2. day bed
  3. night bed
  4. rain
  5. tension
  6. menu theme
  7. second calm track (optional)
- ElevenLabs rules:
  - measure the real cost with `check_subscription` first;
  - at most one retry per sound;
  - every take goes in the ledger;
  - a person approves each take before import.
- Campfire and thunder are made in code, so they cost no credits.

## Verification

- `go test ./pkg/sfxsynth ./pkg/audiomanifest ./cmd/audiogen`:
  - determinism, variants, normalization, seamless loops, WAV round trip;
  - every recipe renders audibly with no NaN;
  - sound-list validation.
- `make audio-check`: 26 sounds, 3.4 MB.
- The import steps were dry-run in a scratch folder (loudnorm, 48 kHz, loopfix, Vorbis).

## Next

1. Restart the editor so it inherits `ELEVEN_LABS_API_KEY`, approve `elevenlabs` in `/mcp`, and generate in priority order.
2. Run `scripts/audio_import.sh <raw> <file> <credits> "<license>"` for each approved take.
3. Add the music and ambience credit line to Credits and THIRD_PARTY_NOTICES.md.

## Update: ambience, music and the camera fade (2026-10-21)

- **Ambience (ElevenLabs, free tier):** day, night and rain beds, each stitched from four 5 s clips (the sound-effects tool's limit) into a 16 s seamless loop, normalized to about -20 LUFS. The beds cost 600 credits (about 50 per clip; the API counter lagged and showed 300). All 12 takes are in the ledger; ElevenLabs is credited on the Credits screen and in THIRD_PARTY_NOTICES.md (non-commercial).
- **Music:** ElevenLabs refused music on the free tier (HTTP 402 `paid_plan_required`, no credits spent), so the music is composed in code:
  - `pkg/musicsynth`: Karplus-Strong guitar, detuned-saw pads and strings with vibrato and tremolo, a brassy horn, bass, frame drum, taiko and shaker, a Schroeder reverb, RMS mastering with a soft limiter, and tails that wrap so loops ring into their head.
  - `cmd/audiogen/music.go`: three scores.
    - Theme (211 s, loops): a guitar motif, taken up by horn and strings, a swell with drums at 1:31-2:29, then back to solo guitar. -16.3 LUFS, LRA 9.5.
    - Calm (180 s): fingerpicked guitar, a pad and a pentatonic melody. -16.4 LUFS.
    - Tension (74 s, loops): string ostinato, frame drum sixteenths, taiko, tremolo cluster and horn stabs. -18.4 LUFS, LRA 0.8.
  - `make music` renders them in about 10 s and encodes with ffmpeg. They are listed with source `composed`; `make audio` keeps them.
- **Import fix:** `scripts/audio_import.sh` now measures loudness and applies the exact gain with a limiter (single-pass loudnorm undershot short, quiet clips by 2-8 dB).
- **Totals:** 32 sounds, 10.2 MB of the 20 MB budget.
- **Camera fade:** see SND-001 (`zoomEar`).

