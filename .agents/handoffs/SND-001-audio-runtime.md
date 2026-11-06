# Task Handoff

Task ID: SND-001
Status: complete (code); music and ambience files pending (SND-002)

## Summary

The game has sound. The UI adapter decides what is heard and sends it through a `Sound` port it owns. A new outbound adapter plays it with Ebitengine audio. Without the adapter (tests, `-capture`, `-noaudio`) the client is silent and opens no audio device.

## Files Changed

- `internal/adapters/inbound/ebiten/`:
  - `sound.go`: the `Sound` port and the silent `noSound`
  - `sound_signals.go`: renderer cues
  - `sound_director.go`: placing, limiting, ambience and music
  - `app.go`: `AppConfig.Sound`, `soundFrame` each Update, click sounds
- Renderer hooks:
  - `work_anim.go`: strike sounds
  - `renderer.go`: `listenWorld`, thunder, speed keys
  - `combat_anim.go`: traces
  - `threat_ui.go`: the threat sting and the start of tension
  - `environment.go`: thunder delay after a flash
- `internal/adapters/outbound/ebitenaudio/` (new): engine, loader, device backend, PCM helpers, tests, architecture test, AGENTS.md
- `cmd/game/main.go`: `-noaudio`, `RIMWORLD_GO_NOAUDIO=1`, `openSound`
- `go.mod`: oto/v3, oggvorbis and vorbis (indirect, through ebiten audio)

## Public Contracts Changed

`AppConfig.Sound`. Nil means silent.

## Invariants / Decisions

- Only `ebitenaudio` imports `ebiten/v2/audio`. `cmd/simulate` never depends on audio (`arch_test.go`).
- Cues come from presentation signals only:
  - strikes from `workStrikeAt`, so a paused clock is silent and 3x speed strikes three times as often;
  - world diffs: frames finished, trees cut, mines done, door occupancy;
  - traces, lightning (thunder 20–90 frames later), new threats and notices, clicks and speed keys.
- Placement: pan up to ±0.8 by screen x. Volume fades to silence half a view off screen.
- Camera distance (`zoomEar`): effects fade from full at zoom 1.5 to silence at the map's zoom-out limit (`minZoomFor`), equal steps per halving of the zoom. They also narrow in stereo (to half width) and thin out (3, then 2, then 1 copy of a sound per frame). Campfires fade with them; ambience beds, music, UI and thunder don't.
- Limits:
  - 4 frames between plays of one id;
  - 3 copies of an id per frame;
  - the 4 nearest cues at the fastest speed;
  - 24 voices in the engine, the oldest one stolen.
- Ambience:
  - `amb.day` and `amb.night` crossfade at dawn and dusk; no crickets in winter or at 8 °C or below;
  - `amb.rain` follows rain and storms;
  - × 0.4 when the camera centre is indoors;
  - the 2 nearest burning fires crackle in stereo;
  - the menu plays a low night bed and the campfire.
- Music:
  - menu (calm until a menu theme exists);
  - calm, with 40–150 s of silence between plays;
  - tension during a raid and for 45 s after it (only once a tension track exists);
  - × 0.6 under the pause menu;
  - 2.5 s crossfades.
- A missing id is silent and logged once; it never panics.

## Verification

Tests:
- `sound_test.go`:
  - strike cue once per swing, silent when paused;
  - silent actions;
  - pan and culling;
  - repeat limits;
  - door and tree cues;
  - music following threats;
  - menu fallback;
  - night ambience;
  - click sounds;
  - `dayWeight`.
- `ebitenaudio` (16 tests, fake backend, no device):
  - gain, pan bytes, voice cap, fades, crossfades, music gaps;
  - loading the real sound list;
  - the architecture test.

Not yet done: listening in the running game (`go run ./cmd/game`). The device smoke test is behind `-tags audiodevice`.

## Determinism Impact

None. Sound reads presentation state only and uses its own RNG.

## Known Limitations

- No pitch or filter control in Ebitengine. Variety comes from 3 pre-made variants, and the indoor effect is volume only.
- OGG voices are not panned (music and beds are not positional).

## Addendum: animal voices (SND-003 contract)
- **Ids:** `animal.<roster key>.<event>`, with events `call`, `attack`, `hurt` and `die`; plus `animal.burst`.
- **Fallback chain:**
  1. `animal.<key>.<event>`
  2. `animal.<group>.<event>`, where the group is one of `small_mammal`, `large_mammal`, `canid`, `felid`, `bovid`, `bird_small`, `bird_large`, `reptile`, `insect` (`voiceGroup` in `animal_sounds.go`)
  3. `animal.generic.<event>`

  The chain is resolved with `Sound.Has` (`Renderer.soundHas`).
- **Triggers** (presentation, `animal_sounds.go`):
  - **Calls** while wandering come from per-animal timers: 25–70 s, scaled by game speed, none while paused, on screen only. They're capped at 2 calls a second and 3 species per 10 s.
  - A **cry** sounds once per flee.
  - **Attack** and **hurt** come from combat traces, with `sfx.melee_hit` kept under them at 0.6.
  - **Die** plays when an animal leaves the world and a corpse of its species is on its tile.
  - **Burst** plays from `WorldView.Bursts`.
- **Files:**
  - ElevenLabs takes become `generated/audio/sfx/animals/<key>_<event>[_b].ogg`, made by `cmd/sfxgen` and `make animal-audio` (ffmpeg plus oggenc).
  - Procedural group placeholders come from `cmd/audiogen`.
- **Engine:** OGG one-shots are decoded once, cached (LRU, 24 MiB of PCM) and panned like WAV.
