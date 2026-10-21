package ebitenaudio

import (
	"io"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// player is one voice on the device (*audio.Player in the game, a fake in
// tests).
type player interface {
	Play()
	IsPlaying() bool
	SetVolume(float64)
	Close() error
}

// backend makes voices from 48 kHz 16-bit little-endian stereo PCM.
type backend interface {
	fromBytes(stereo16 []byte) player
	fromStream(r io.Reader) (player, error)
}

// oneShotBuffer keeps one-shots tight to the action that cues them.
const oneShotBuffer = 35 * time.Millisecond

// device is the real backend over the process's Ebitengine audio context.
type device struct{ ctx *audio.Context }

func newDevice() device {
	ctx := audio.CurrentContext()
	if ctx == nil {
		ctx = audio.NewContext(sampleRate)
	}
	return device{ctx: ctx}
}

func (d device) fromBytes(stereo16 []byte) player {
	p := d.ctx.NewPlayerFromBytes(stereo16)
	p.SetBufferSize(oneShotBuffer)
	return p
}

func (d device) fromStream(r io.Reader) (player, error) {
	p, err := d.ctx.NewPlayer(r)
	if err != nil {
		return nil, err
	}
	return p, nil
}
