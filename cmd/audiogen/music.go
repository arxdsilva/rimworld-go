package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"rimworld-go/pkg/audiomanifest"
	m "rimworld-go/pkg/musicsynth"
	"rimworld-go/pkg/sfxsynth"
)

// The game's music, composed in code (SND-002): an epic main theme for the
// menus, a calm colony track, and a tension loop for raids. All in A minor,
// frontier colours: plucked guitar, strings, horn, frame drums.

const composedLicense = "original, composed in code (CC0, this repo)"

// track is one piece of music and its place in the sound list.
type track struct {
	id, file string
	loop     bool
	volume   float64
	seed     uint64
	score    func(seed uint64) m.Score
}

var tracks = []track{
	{"music.menu", "music/theme.ogg", true, 0.75, 901, themeScore},
	{"music.calm", "music/calm_1.ogg", false, 0.7, 902, calmScore},
	{"music.tension", "music/tension.ogg", true, 0.75, 903, tensionScore},
}

// comp places notes on a beat grid.
type comp struct {
	bpm   float64
	notes []m.Note
	r     uint64 // humanizing hash state
}

func (c *comp) at(beat float64) float64 { return beat * 60 / c.bpm }

// jitter is a small deterministic offset in [-1, 1).
func (c *comp) jitter() float64 {
	c.r = c.r*6364136223846793005 + 1442695040888963407
	return float64(c.r>>11)/(1<<52) - 1
}

func (c *comp) add(inst m.Inst, beat, beats float64, pitch int, vel, pan float64) {
	t := c.at(beat)
	if inst == m.Guitar || inst == m.Shaker {
		t += 0.008 * c.jitter() // a player, not a sequencer
		vel *= 1 + 0.08*c.jitter()
	}
	c.notes = append(c.notes, m.Note{Inst: inst, Start: max(0, t), Dur: c.at(beats), Pitch: pitch, Vel: vel, Pan: pan})
}

// chord voicings: guitar (5 strings, low to high), pad (3 notes), root.
type chord struct {
	guitar []int
	pad    []int
	root   int
}

var (
	chAm    = chord{[]int{45, 52, 57, 60, 64}, []int{57, 60, 64}, 45}
	chF     = chord{[]int{41, 48, 53, 57, 60}, []int{53, 57, 60}, 41}
	chFmaj7 = chord{[]int{41, 48, 52, 57, 64}, []int{52, 57, 60}, 41}
	chC     = chord{[]int{48, 52, 55, 60, 64}, []int{55, 60, 64}, 48}
	chG     = chord{[]int{43, 50, 55, 59, 62}, []int{55, 59, 62}, 43}
)

// arp picks a chord's guitar strings as eighth notes over one bar.
func (c *comp) arp(bar int, ch chord, pattern []int, vel, pan float64) {
	for i, s := range pattern {
		c.add(m.Guitar, float64(bar*4)+float64(i)*0.5, 1.5, ch.guitar[s], vel, pan)
	}
}

// strum rings a whole chord, low to high, from beat.
func (c *comp) strum(beat, beats float64, ch chord, vel float64) {
	for i, p := range ch.guitar {
		c.add(m.Guitar, beat+float64(i)*0.06, beats, p, vel, -0.1+0.05*float64(i))
	}
}

// phrase is a melody: (beat offset, pitch, beats).
type phrase [][3]float64

// The theme's motif: a rising call (motifA) and its answer home (motifB),
// four bars each over Am-F-C-G.
var (
	motifA = phrase{{0, 76, 1.5}, {1.5, 74, 0.5}, {2, 72, 2}, {4, 69, 3}, {7, 72, 1},
		{8, 74, 1.5}, {9.5, 76, 0.5}, {10, 79, 2}, {12, 76, 4}}
	motifB = phrase{{0, 76, 1.5}, {1.5, 74, 0.5}, {2, 72, 1}, {3, 71, 1}, {4, 69, 2}, {6, 67, 2},
		{8, 65, 2}, {10, 67, 1}, {11, 69, 1}, {12, 69, 4}}
)

func (c *comp) play(inst m.Inst, bar int, p phrase, shift int, vel, pan float64) {
	for _, n := range p {
		c.add(inst, float64(bar*4)+n[0], n[2], int(n[1])+shift, vel, pan)
	}
}

// themeScore is the main theme: a lone guitar motif at dusk, taken up by
// strings and horn, a full swell with drums, easing back to the guitar,
// which leads straight back into its own start when it loops.
func themeScore(seed uint64) m.Score {
	c := &comp{bpm: 84, r: seed}
	const bars = 74
	prog := []chord{chAm, chF, chC, chG}
	arp := []int{0, 1, 2, 3, 4, 3, 2, 1}
	for bar := 0; bar < bars; bar++ {
		ch := prog[bar%4]
		switch {
		case bar == bars-1:
			c.strum(float64(bar*4), 4, ch, 0.5)
		case bar >= 32 && bar < 52:
			c.arp(bar, ch, arp, 0.32, -0.15)
		default:
			c.arp(bar, ch, arp, 0.5, -0.15)
		}
		if bar >= 16 && bar < 64 { // the string pad under everything in the middle
			vel := 0.33
			switch {
			case bar >= 32 && bar < 52:
				vel = 0.5
			case bar >= 52:
				vel = 0.28
			}
			for i, p := range ch.pad {
				c.add(m.Pad, float64(bar*4), 4, p, vel, -0.4+0.4*float64(i))
			}
		}
		if bar >= 32 && bar < 52 { // the swell: drums and bass
			grow := min(1, 0.55+0.15*float64(bar-32)) // a crescendo into it
			c.add(m.Frame, float64(bar*4), 0.5, 0, 0.7*grow, 0.2)
			c.add(m.Frame, float64(bar*4)+2, 0.5, 0, 0.6*grow, 0.2)
			c.add(m.Frame, float64(bar*4)+3.5, 0.5, 0, 0.35*grow, 0.25)
			if bar%2 == 0 {
				c.add(m.Taiko, float64(bar*4), 1, 0, 0.85*grow, 0)
			}
			c.add(m.Bass, float64(bar*4), 2, ch.root-12, 0.55*grow, 0)
			c.add(m.Bass, float64(bar*4)+2, 2, ch.root-12, 0.45*grow, 0)
			c.add(m.Horn, float64(bar*4), 4, ch.pad[1], 0.32*grow, -0.3) // a held counter-line
		}
	}
	// the melody passes from guitar to horn to violins and back
	c.play(m.Guitar, 8, motifA, 0, 0.62, 0.15)
	c.play(m.Guitar, 12, motifB, 0, 0.6, 0.15)
	for i, b := range []int{16, 20, 24, 28} {
		mo := motifA
		if i%2 == 1 {
			mo = motifB
		}
		c.play(m.Horn, b, mo, -12, 0.55, 0.2)
	}
	for i, b := range []int{32, 36, 40, 44} {
		mo := motifA
		if i%2 == 1 {
			mo = motifB
		}
		c.play(m.Strings, b, mo, 0, 0.62, 0.1)
		c.play(m.Strings, b, mo, -12, 0.4, -0.2)
	}
	c.play(m.Strings, 48, motifA, 12, 0.5, 0.1) // the peak, an octave up
	c.play(m.Strings, 52, motifA, 0, 0.42, 0.1)
	c.play(m.Strings, 56, motifB, 0, 0.36, 0.1)
	c.play(m.Guitar, 68, motifB, 0, 0.55, 0.15)
	return m.Score{Length: c.at(bars * 4), Notes: c.notes, Reverb: 0.32, Seed: seed}
}

// pentatonic is A minor pentatonic over two octaves.
var pentatonic = []int{57, 60, 62, 64, 67, 69, 72, 74, 76}

// calmScore is the colony track: fingerpicked guitar over a soft pad,
// a wandering pentatonic melody and a light shaker, ending gently.
func calmScore(seed uint64) m.Score {
	c := &comp{bpm: 72, r: seed}
	const bars = 54
	prog := []chord{chFmaj7, chC, chG, chAm}
	pick := []int{0, 2, 3, 2, 1, 2, 4, 2}
	walk := 4
	for bar := 0; bar < bars; bar++ {
		ch := prog[bar%4]
		if bar == bars-1 {
			c.strum(float64(bar*4), 4, chAm, 0.42)
			break
		}
		vel := 0.42
		if bar >= bars-4 {
			vel = 0.42 * float64(bars-bar) / 5
		}
		c.arp(bar, ch, pick, vel, -0.1)
		if bar >= 8 && bar < 50 {
			for i, p := range ch.pad {
				c.add(m.Pad, float64(bar*4), 4, p, 0.24, -0.35+0.35*float64(i))
			}
		}
		if bar >= 16 && bar < 46 {
			c.add(m.Bass, float64(bar*4), 2, ch.root-12, 0.36, 0)
			c.add(m.Bass, float64(bar*4)+2, 2, ch.root-12, 0.28, 0)
			for e := 1; e < 8; e += 2 {
				c.add(m.Shaker, float64(bar*4)+float64(e)*0.5, 0.25, 0, 0.18, 0.35)
			}
		}
		// a melody in every other four-bar phrase, stepping around the scale
		if bar >= 12 && bar < 48 && (bar/4)%2 == 1 {
			inst := m.Guitar
			if (bar/8)%2 == 0 {
				inst = m.Strings
			}
			for beat := 0.0; beat < 4; {
				n := 1.0 + float64(int(c.jitter()+1))
				walk = max(0, min(len(pentatonic)-1, walk+int(c.jitter()*2.5)))
				vel := 0.36
				if inst == m.Guitar {
					vel = 0.5
				}
				c.add(inst, float64(bar*4)+beat, n, pentatonic[walk]+12*boolInt(inst == m.Guitar), vel, 0.2)
				beat += n
			}
		}
	}
	return m.Score{Length: c.at(bars * 4), Notes: c.notes, Reverb: 0.3, Seed: seed}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// tensionScore is the raid loop: a pulsing low string ostinato, frame
// drum sixteenths, taiko hits, a dissonant high tremolo and horn stabs,
// the same intensity throughout so it loops seamlessly.
func tensionScore(seed uint64) m.Score {
	c := &comp{bpm: 110, r: seed}
	const bars = 34
	ostinato := []int{33, 33, 45, 33, 34, 33, 45, 31}
	drums := "X..xX.x.X..xX.xx"
	for bar := 0; bar < bars; bar++ {
		b := float64(bar * 4)
		for i, p := range ostinato {
			c.add(m.Strings, b+float64(i)*0.5, 0.45, p, 0.55, -0.2)
		}
		for i, s := range drums {
			switch s {
			case 'X':
				c.add(m.Frame, b+float64(i)*0.25, 0.25, 0, 0.62, 0.25)
			case 'x':
				c.add(m.Frame, b+float64(i)*0.25, 0.25, 0, 0.3, 0.3)
			}
		}
		c.add(m.Taiko, b, 1, 0, 0.9, 0)
		c.add(m.Taiko, b+2.5, 1, 0, 0.55, 0)
		if bar%4 == 0 {
			c.add(m.Tremolo, b, 16, 76, 0.24, 0.35)
			c.add(m.Tremolo, b, 16, 77, 0.2, 0.45)
		}
		if bar%8 == 0 {
			c.add(m.Horn, b, 2, 45, 0.55, -0.1)
			c.add(m.Horn, b, 2, 52, 0.45, 0.1)
		}
	}
	return m.Score{Length: c.at(bars * 4), Notes: c.notes, Reverb: 0.22, Seed: seed}
}

// composeAll renders every track, encodes Ogg Vorbis with ffmpeg and lists
// it in the sound list.
func composeAll(assets string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("-music needs ffmpeg (with libvorbis) to encode Ogg: %w", err)
	}
	mf, err := readManifest(assets)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "audiogen-music-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, t := range tracks {
		l, r := m.Render(t.score(t.seed))
		pcm := m.Master(l, r, 0.13, 0.89, t.seed)
		wav := filepath.Join(tmp, filepath.Base(t.file)+".wav")
		var buf bytes.Buffer
		if err := sfxsynth.WriteWAVChannels(&buf, pcm, m.Rate, 2); err != nil {
			return err
		}
		if err := os.WriteFile(wav, buf.Bytes(), 0o644); err != nil {
			return err
		}
		out := filepath.Join(assets, filepath.FromSlash(audiomanifest.Dir), filepath.FromSlash(t.file))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", wav,
			"-c:a", "libvorbis", "-q:a", "4", out)
		if b, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("encode %s: %v: %s", t.file, err, b)
		}
		e := audiomanifest.Entry{ID: t.id, Bus: audiomanifest.BusMusic, Loop: t.loop, Volume: t.volume,
			Files: []string{t.file}, Source: audiomanifest.SourceComposed, Recipe: t.id, Seed: t.seed,
			Seconds: float64(len(l)) / m.Rate, License: composedLicense}
		replaced := false
		for i := range mf.Sounds {
			if mf.Sounds[i].ID == t.id {
				mf.Sounds[i], replaced = e, true
			}
		}
		if !replaced {
			mf.Sounds = append(mf.Sounds, e)
		}
		fmt.Printf("audiogen: %s %s (%.0f s)\n", t.id, t.file, e.Seconds)
	}
	if err := mf.Validate(); err != nil {
		return err
	}
	return writeManifest(assets, mf)
}
