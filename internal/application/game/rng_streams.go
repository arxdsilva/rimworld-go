package game

import (
	"encoding/binary"
	"slices"

	"rimworld-go/pkg/randx"
)

// Random streams of the social and host systems (ENG-003 C0). Each is
// randx.New(seed ^ salt), created on first use and saved, and never g.rng:
// drawing from one never shifts the main stream, so a system that does
// nothing leaves every other outcome alone.
const (
	socialSalt = 0x50C1A1
	bodySalt   = 0xB0D1E5
	drugSalt   = 0xD2065
	worldSalt  = 0x30E1D
	prisonSalt = 0x9215
	bondSalt   = 0xB04D
	powerSalt  = 0x90E12
	// medSalt (MED-00): tend quality, infection, disease and surgery
	// rolls of the medical system.
	medSalt = 0x3ED1CA
	// PLAY-024: trader stock, prices' rolls and drop pods (tradeSalt);
	// sentry turret shots and blasts (defenseSalt).
	tradeSalt   = 0x7EAD3
	defenseSalt = 0xDEF3A5
	craftSalt   = 0xC2AF7 // PRD-2: quality rolls and worn apparel wear
)

// Stream names, the RNGStreams save keys.
const (
	streamSocial  = "social"
	streamBody    = "body"
	streamDrug    = "drug"
	streamWorld   = "world"
	streamPrison  = "prison"
	streamBond    = "bond"
	streamPower   = "power"
	streamMedical = "medical"
	streamTrade   = "trade"
	streamDefense = "defense"
	streamCraft   = "craft"
)

// streamSeedKey holds the base seed in the RNGStreams save map (8 bytes,
// little endian), so a stream first used after a load starts where it
// would have in the unbroken game.
const streamSeedKey = "seed"

var streamSalts = map[string]uint64{
	streamSocial: socialSalt, streamBody: bodySalt, streamDrug: drugSalt,
	streamWorld: worldSalt, streamPrison: prisonSalt, streamBond: bondSalt,
	streamPower: powerSalt, streamMedical: medSalt,
	streamTrade: tradeSalt, streamDefense: defenseSalt, streamCraft: craftSalt,
}

// rngStreams is the base seed and the streams created so far.
type rngStreams struct {
	seed uint64
	m    map[string]*randx.Rand
}

// stream is the named stream, created from the base seed on first use.
func (g *Game) stream(name string) *randx.Rand {
	if r, ok := g.streams.m[name]; ok {
		return r
	}
	salt, ok := streamSalts[name]
	if !ok {
		panic("game: unknown random stream " + name)
	}
	if g.streams.m == nil {
		g.streams.m = make(map[string]*randx.Rand)
	}
	r := randx.New(g.streams.seed ^ salt)
	g.streams.m[name] = r
	return r
}

// exportStreams is the RNGStreams save field: the base seed and each
// stream's state (nil while no stream was ever used).
func (g *Game) exportStreams() (map[string][]byte, error) {
	if len(g.streams.m) == 0 && g.streams.seed == 0 {
		return nil, nil
	}
	out := make(map[string][]byte, len(g.streams.m)+1)
	out[streamSeedKey] = binary.LittleEndian.AppendUint64(nil, g.streams.seed)
	names := make([]string, 0, len(g.streams.m))
	for n := range g.streams.m {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		b, err := g.streams.m[n].MarshalBinary()
		if err != nil {
			return nil, err
		}
		out[n] = b
	}
	return out, nil
}

// importStreams restores exportStreams' state (nil: an older save, base
// seed 0 and no streams yet).
func (g *Game) importStreams(saved map[string][]byte) error {
	g.streams = rngStreams{}
	if b := saved[streamSeedKey]; len(b) == 8 {
		g.streams.seed = binary.LittleEndian.Uint64(b)
	}
	for n, b := range saved {
		if n == streamSeedKey {
			continue
		}
		if _, ok := streamSalts[n]; !ok {
			continue // a stream this version does not know
		}
		r := randx.New(0)
		if err := r.UnmarshalBinary(b); err != nil {
			return err
		}
		if g.streams.m == nil {
			g.streams.m = make(map[string]*randx.Rand)
		}
		g.streams.m[n] = r
	}
	return nil
}
