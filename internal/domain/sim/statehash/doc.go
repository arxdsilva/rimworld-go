// Package statehash computes stable FNV-64a fingerprints of simulation
// state, so golden tests can pin a run's outcome and catch any change in
// behaviour. Hashes are canonical: map entries are ordered by their
// encoded keys, so Go's random map iteration never changes a hash.
package statehash
