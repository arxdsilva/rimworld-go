package ebiten

import "rimworld-go/internal/platform/config"

// Sound is where the client sends audio (cmd/game wires the Ebitengine
// audio adapter, internal/adapters/outbound/ebitenaudio). Ids, buses,
// variants and loudness come from the sound list
// (assets/metadata/audio/audio.json). This package never imports an audio
// library, so tests and captures stay silent and need no audio device.
type Sound interface {
	// Has reports whether id is in the sound list (fallbacks for sounds
	// not made yet).
	Has(id string) bool
	// Play starts a one-shot at vol (0..1) and stereo pan (-1..1).
	Play(id string, vol, pan float64)
	// Loop keeps emitter key playing id at a target vol and pan, fading
	// to them; vol 0 fades it out and frees it.
	Loop(key, id string, vol, pan float64)
	// Music crossfades to track ("" fades to silence) at vol. A looping
	// track repeats; a non-looping one plays its files in turn with a
	// silence between plays.
	Music(track string, vol float64)
	SetVolumes(config.Audio)
	// Update runs once per frame: fades, finished voices, focus mute.
	Update()
}

// noSound is the silent Sound (tests, captures, -noaudio).
type noSound struct{}

func (noSound) Has(string) bool                       { return false }
func (noSound) Play(string, float64, float64)         {}
func (noSound) Loop(string, string, float64, float64) {}
func (noSound) Music(string, float64)                 {}
func (noSound) SetVolumes(config.Audio)               {}
func (noSound) Update()                               {}
