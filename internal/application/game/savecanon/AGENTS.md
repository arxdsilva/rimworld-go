# internal/application/game/savecanon

## Purpose
SIM-074 canonicalizes a standard Gob stream after game exports its authoritative snapshot. Equal supported values encode identically regardless of map iteration or prior type numbering.

## Owns
Frozen (ownership transferred by the caller after typed detachment), Capture, Encode and its private canonicalization step; the private Gob reader, wire-type descriptors and value writers. Sort encoded map keys and number user types in canonical first-use order. This is not another save schema or runtime store.

## Contracts
The standard Gob decoder recovers the same values. Unsupported interface values and invalid/truncated streams return errors. Preserve input immutability and bounds checks. Keep this work outside ticks and authoritative StateHash. Frozen must never hold a live Game, a provider or callback, or borrowed mutable state. The save worker encodes and recycles only its own buffers.

## Tests
TestCanonicalIsOneByteStringAndDecodesToTheSameValue and TestCorruptStreamIsAnErrorNotAPanic pin the wire contract. Root save tests cover raw/gzip, historical v12 and multi-map byte identity.
