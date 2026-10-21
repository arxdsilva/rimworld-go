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
