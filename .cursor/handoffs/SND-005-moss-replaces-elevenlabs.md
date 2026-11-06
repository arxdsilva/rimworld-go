# Task Handoff

Task ID: SND-005
Status: done (2026-11-06). The user approved every take after the review page and asked for the ambience to ship.

## Summary
The ElevenLabs free-tier sounds were non-commercial only, so they blocked an open licence. Every one is now replaced with a MOSS-SoundEffect v2.0 take generated locally:
- 281 animal voices (61 species, 9 voice groups, the hive burst), 491 files with variants.
- The three ambience beds: day, night and rain.

No ElevenLabs output ships any more, and `make audio-check` keeps it that way.

## Files changed
- `cmd/sfxgen`:
  - `-supersede elevenlabs` makes approved ElevenLabs sounds again (b29aec2b2).
  - The ledger records diffusion steps per take (e90c661ac).
  - `generate.py` `crop()` cuts a short sound from its onset when the model starts it late in the 30 s window it always makes (ba1235c76). Takes that already start on time come out bit-identical.
  - `-approve` of an ElevenLabs take now needs `-license` with the paid plan's terms. The free-tier constant is gone.
- `pkg/audiomanifest`: `Validate` (and with it `Decode`, `-approve`, `-register` and `make audio-check`) rejects a license line containing "non-commercial". `NonCommercial` is exported, and `TestValidateRejects` covers it.
- `cmd/audiogen -register -seed N`, together with `scripts/audio_import.sh … <seed>`, lists an imported MOSS take as `source: moss` with its seed and no credits. A take from another backend replaces the old files. Covered by `cmd/audiogen/register_test.go`.
- Assets:
  - `assets/generated/audio/sfx/animals/*.ogg` and `ambience/{day,night,rain}.ogg`.
  - `assets/metadata/audio/audio.json`: 300 MOSS entries, no ElevenLabs entries.
  - The ledger: every MOSS take, approved by arxdsilva.
- `assets/source/audio/animal_prompts.json`: `hare`, `cougar`, `reptile` and `fennec_fox` hurt prompts reworded ("…loudly in pain"). The earlier wording kept producing only a click.
- Credits screen (`mainmenu.CreditsLines`), `THIRD_PARTY_NOTICES.md`, `assets/AGENTS.md`, `docs/FEATURES.md`, `docs/DEVELOPING.md`, ART §25.1, skill `rimworld-go-creatures` and the `audio_import.sh` header now name MOSS.

## Generation notes
- The run used 50 diffusion steps (`-moss-steps 50`), about 105 s per take. 100 steps took about 230 s and sounded the same.
- The pipeline always denoises 30 s. Shorter windows were tried and failed: 10 s came out silent and 3 s came out as noise.
- A second parallel process gave no speed-up, because the GPU is already saturated.
- The ambience beds are 20 s takes at seed 7 and 100 steps, from the same prompts as before.

## Not done here
- The licence itself (GPL-3.0 code, CC BY-SA 4.0 assets) comes next.
- The ledger keeps the old ElevenLabs takes as a record; history keeps the old files under their own terms.

## Tests
- `make audio-check`: 403 sounds ok, 24.7 MB of 30 MB.
- `make art-check` passed.
- `go test -race ./pkg/audiomanifest/ ./cmd/sfxgen/ ./cmd/audiogen/ ./internal/adapters/inbound/ebiten/render/cue/ ./internal/adapters/inbound/ebiten/mainmenu/ ./internal/arch/` passed.
- The full suite passed (`make test`).
