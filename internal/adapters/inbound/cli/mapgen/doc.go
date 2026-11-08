// Package mapgen is the mapgen command: it renders the MAPGEN-2 field generator's debug images: one
// PNG per field, a metrics file, contact sheets of seeds, a hills sweep, or
// the site of an exact world tile.
//
//	go run ./cmd/mapgen -seed 1001 -biome temperate_forest -hills large -out /tmp/m
//	go run ./cmd/mapgen -grid 12 -seed 1001 -out /tmp/g
//	go run ./cmd/mapgen -sweep hills -seed 1001 -out /tmp/s
//	go run ./cmd/mapgen -tile amber-hollow:1234 -out /tmp/t
package mapgen
