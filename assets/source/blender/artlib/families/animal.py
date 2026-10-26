"""Animals (ART spec §11, §11.1): the family entry point.

Iterates the binding roster (animal_roster.py) and hands each species to
the builder registered for its archetype (families/animals/). The
builders share one rig (animals/rig.py): a parametric Body spec, posed at
build time, rendered in three facings (south, east, north; the renderer
mirrors east for west) with trot, attack, graze and flinch frames, plus
three corpse poses.

Environment (for quick iteration):
    ANIMALS=deer,cow          render only these roster keys
    ANIMAL_FRAMES=static      standing frames only (no animation, no corpses)
"""
import os
import time

from . import animal_roster as R
from . import animals


def _selected():
    want = [k.strip() for k in os.environ.get("ANIMALS", "").split(",") if k.strip()]
    unknown = sorted(set(want) - set(R.BY_KEY))
    if unknown:
        raise ValueError("ANIMALS: unknown roster keys %s" % unknown)
    return [r for r in R.ROSTER if not want or r.key in want]


def build():
    static = os.environ.get("ANIMAL_FRAMES", "").strip() == "static"
    builders = animals.load()
    out = []
    for row in _selected():
        fn = builders.get(row.archetype)
        if fn is None:
            print("animal: no builder for %s (%s) yet; skipped" % (row.key, row.archetype))
            continue
        t = time.time()
        made = fn(row, static=static)
        print("animal: %-18s %4d sprites %6.1fs" % (row.key, len(made), time.time() - t))
        out += made
    return out
