"""Shared corner model for linked families (walls, rock, ore).

Cardinal mask bits N=1, E=2, S=4, W=8; corner bits NE=1, SE=2, SW=4, NW=8.
A corner is *relevant* when both of its cardinals link, and *filled* when
its diagonal neighbour links too. Each linked family bakes one sprite per
(mask, subset of relevant corners filled): 47 in all.
"""
CORNERS = (("NE", 1, "N", "E"), ("SE", 2, "S", "E"), ("SW", 4, "S", "W"), ("NW", 8, "N", "W"))
CORNER_BIT = {name: bit for name, bit, _, _ in CORNERS}
BITS = (("N", 1), ("E", 2), ("S", 4), ("W", 8))


def relevant(mask):
    """Names of the corners whose two cardinals both link."""
    con = {d for d, b in BITS if mask & b}
    return [n for n, _, a, b in CORNERS if a in con and b in con]


def _variants(mask):
    """Yield (filled corner names, key suffix) for every variant of mask.
    Suffix is "" when nothing is filled, else "_c<hex of filled bits>"."""
    rel = relevant(mask)
    for f in range(1 << len(rel)):
        filled = {rel[i] for i in range(len(rel)) if f >> i & 1}
        bits = sum(CORNER_BIT[n] for n in filled)
        yield filled, ("_c%x" % bits) if bits else ""


def all_variants():
    """(mask, filled, suffix) over all 47 variants, in key order."""
    for mask in range(16):
        for filled, suffix in _variants(mask):
            yield mask, filled, suffix


def corner_seed(filled):
    return sum(CORNER_BIT[n] for n in filled)
