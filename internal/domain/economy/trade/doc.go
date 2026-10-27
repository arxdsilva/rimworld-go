// Package trade is the pure trading model of PLAY-024: trader kinds and
// their stock, buy and sell prices in hundredths of a silver, and settling
// an offer against the colony's goods, the trader's stock and silver, and
// the negotiation terms. It is headless and deterministic: randomness comes
// in as an explicit *randx.Rand, and nothing here knows about maps, pawns
// or the clock. Tuning lives in .cursor/PLAY024_PLAN.md (original numbers).
package trade
