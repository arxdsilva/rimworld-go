# Frontier Lives: an example script (PLG-060). It only talks to the
# player; it changes nothing in the colony, so it has no balance impact.
# The hooks and ctx are listed in README.md beside this file.

# A few lines the frontier mutters; one is picked with ctx.rand, so the
# same colony seed always hears the same line on the same day.
SAYINGS = [
    "Mend the fence before the wind finds the gap.",
    "A full woodpile is a quiet winter.",
    "Count the stars, then count the rations.",
    "Every trail was once somebody's first step.",
]

def on_day(ctx):
    # Every fifth colony day (not the landing day), a word from the frontier.
    if ctx.day > 0 and ctx.day % 5 == 0:
        line = SAYINGS[ctx.rand(len(SAYINGS))]
        ctx.message("Frontier wisdom, day %d of %s: %s" % (ctx.day, ctx.season, line))

def on_incident(ctx, incident):
    # A short, calm note after a threat; nature events pass in silence.
    if incident.category == "threat":
        ctx.message("The frontier is never quiet for long. Stay close to the walls (%d colonists hold the line)." % ctx.colonists)
