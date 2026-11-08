// Package normalgen is the normalgen command: it writes a normal map <name>_n.png next to every world
// sprite under the given directories (ART §6.1), with pkg/spritenormals,
// a port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez); see
// THIRD_PARTY_NOTICES.md. Tint masks, normal maps and UI art are skipped,
// and a sprite whose normal map is newer is left alone unless -force.
// Autotiled sprites (metadata `autotile` + `mask`) are clipped flush where
// they join a neighbour; their silhouette is extended past those edges
// before the bevel is computed, so joined tiles light as one surface.
//
//	normalgen [-dirs assets/generated/world] [-force]
package normalgen
