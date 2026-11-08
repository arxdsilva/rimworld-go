// Package atlasgen is the atlasgen command: it packs one sprite family into texture atlas pages
// (NG-1 plan 7.1, ART spec 24 and 26), so a family with tens of thousands
// of small files loads as a handful of images and costs far less disk.
//
//	atlasgen [-family pawn] [-max 4096] [-root assets] [-check]
//
// It reads assets/metadata/<family>/*.yaml and the PNGs they name (the
// sprite, its tint mask and its normal map `<image>_n.png`), trims every
// sprite to the bounding box of its non-transparent pixels, shelf-packs
// the trimmed rectangles into pages of at most -max x -max pixels in
// sorted key order, and writes
//
//	assets/generated/atlas/<family>_<n>.png        sprite pixels
//	assets/generated/atlas/<family>_<n>_mask.png   tint masks (8-bit grey), same rects
//	assets/generated/atlas/<family>_<n>_n.png      normal maps (RGB, opaque), same rects
//	assets/metadata/atlas/<family>.yaml            key -> page, rect, trim and
//	                                               the sprite's own metadata
//
// The mask and normal pages share the sprite page's rectangles, so a
// sprite is cut out of all three at one place. A normal map has the same
// alpha as its sprite (make art-check enforces it), so its page keeps RGB
// only and the loader takes the alpha from the sprite page. The output is
// deterministic: sorted keys, fixed packing order, fixed PNG encoder.
// -check validates the written atlas instead (pages, rectangles, overlap,
// and that every sprite under metadata/<family> is present).
//
// Atlas file format (the one metadata subset the renderer's loader reads):
//
//	version: 1
//	family: pawn
//	max: 4096
//	pages: 3
//	sprites:
//	  body_south:
//	    page: 0
//	    rect: [x, y, w, h]     # trimmed pixels in the page
//	    trim: [ox, oy]         # where the rect sits in the sprite's canvas
//	    normal: true
//	    id: "core:pawn/body_south"
//	    ...                    # the sprite metadata, image/mask pointing at the pages
package atlasgen
