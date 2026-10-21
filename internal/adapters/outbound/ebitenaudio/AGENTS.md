# ebitenaudio outbound adapter

Plays the game's sound list through Ebitengine's audio package. It is the
concrete `Sound` the Ebitengine client talks to; `cmd/game` wires it in.
Presentation-only: nothing here feeds back into the simulation.

## Purpose

Load `assets/metadata/audio/audio.json` and the files it names under
`assets/generated/audio`, then mix one-shots, positional loops and music
on the audio device at 48 kHz, honoring the player's volume settings.

## Owned concepts

- `Engine` — `Has`, `Play`, `Loop`, `Music`, `SetVolumes`, `Update`
  (exactly the client's `Sound` port; it never imports the client)
- `New(assetsRoot)` and `LocateAudio()` (walk up from cwd to `assets/`)
- loader (`load.go`): manifest first, WAVs as mono 48 kHz PCM, OGGs as
  encoded bytes; bad files are logged and skipped
- gain: cue vol × entry volume × `config.Gain(bus)` × `config.Gain(Master)`,
  0 when `MuteUnfocused` and the window is unfocused
- one-shots: random non-repeating variant, pan baked into stereo bytes,
  24-voice cap (oldest closed), finished voices reaped in `Update`
- loops: per-emitter key, 0.8 s fades, WAV loops panned live (atomic pan
  read on the audio goroutine), OGG loops via `audio.NewInfiniteLoop`
- music: 2.5 s crossfades; looping tracks repeat, others play their files
  in turn with 40–150 s of silence between plays
- the `player`/`backend` seam (`device.go`) that tests replace with fakes

## Allowed dependencies

- `github.com/hajimehoshi/ebiten/v2/audio` (+ `audio/vorbis`), and
  `ebiten.IsFocused` for the focus mute
- `pkg/audiomanifest`, `pkg/sfxsynth` (WAV reading)
- `internal/platform/config` (`Audio`, `Gain`, defaults)
- standard library

## Forbidden dependencies

- `internal/domain`, `internal/application`
- inbound adapters (including `internal/adapters/inbound/ebiten`, whose
  `Sound` interface this package satisfies structurally)
- anything reachable from `cmd/simulate` must not import this package or
  `ebiten/v2/audio` (`arch_test.go` checks)

## Rules

- Tests never call `audio.NewContext`: use `newEngine` with a fake
  backend and a stubbed `decode`. The real device test is behind the
  `audiodevice` build tag.
- `New` reads and validates the manifest before opening the audio
  context; a bad manifest returns an error with no device opened.
- Never panic on unknown ids, missing or corrupt files: log once per id
  and play nothing.
- Every method runs on the game loop; only `loopReader`'s pan is shared
  with the audio goroutine, through `sync/atomic`.
- Presentation RNG only (internal splitmix); never the sim's RNG.
