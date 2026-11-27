# Extension API fixture: one script that uses every extension point but jobs.
# Every observable effect is a ctx.message, a ctx.place or a registered def.

EVENTS = ["quadrum", "year", "research_done", "recipe_done", "map_generated"]

def register_defs():
    return {
        "incident": [
            {"name": "dark_noon", "like": "core.eclipse", "label": "Dark noon", "weight": 1, "cooldown": 1, "min_day": 1000},
            {"name": "dim_noon", "like": "core.eclipse", "label": "Dim noon", "weight": 1, "cooldown": 1, "min_day": 1000},
            {"name": "bad_noon", "like": "core.eclipse", "label": "Bad noon", "weight": 1, "cooldown": 1, "min_day": 1000},
        ],
        "stat_mod": [
            {"name": "hasty", "stat": "work_speed", "mul": 500},
        ],
    }

def validate_defs(defs):
    out = []
    for d in defs:
        cls = getattr(d, "class")
        if cls == "incident" and d.key == "extapi.bad_noon":
            out.append({"def": cls + "/" + d.key, "msg": "bad_noon is not allowed"})
        if cls == "incident" and d.key == "core.eclipse":
            out.append({"def": cls + "/" + d.key, "msg": "eclipses are disliked here"})
    return out

def validate_command(ctx, cmd):
    if cmd.kind == "draft":
        return "no drafting in this colony"
    return None

def run_incident(ctx, incident):
    if incident.key == "extapi.dark_noon":
        ctx.message("replaced " + incident.key)
        return True
    return False

def panel(ctx):
    return {
        "title": "Extapi panel",
        "lines": ["day %d" % ctx.day, "colonists %d" % len(ctx.pawns())],
        "actions": [{"id": "ping", "label": "Ping"}],
    }

def on_action(ctx, action):
    ctx.message("acted " + action)

def query_line(ctx):
    m = ctx.map
    an = ctx.animals()
    total = 0
    for a in an:
        total += a.count
    return "q %dx%d %s %s %s an=%d/%d bld=%d fac=%d sites=%d" % (
        m.width, m.height, m.weather, m.terrain(3, 3), m.temperature(3, 3),
        len(an), total, len(ctx.buildings()), len(ctx.world.factions()), len(ctx.world.sites()))

def on_event(ctx, event):
    k = event.kind
    if k == "quadrum":
        ctx.message("ev quadrum")
    elif k == "year":
        ctx.message("ev year")
    elif k == "research_done":
        ctx.message("ev research_done " + event.research)
        ctx.message(query_line(ctx))
    elif k == "recipe_done":
        ctx.message("ev recipe_done %s %s %d" % (event.key, event.product, event.count))
        ctx.place("core.wood", 3, event.x, event.y + 1)
    elif k == "map_generated":
        ctx.message("ev map_generated %d %dx%d %s" % (event.seed, event.width, event.height, event.biome))
        cx = event.width // 2
        cy = event.height // 2
        for i in range(5):
            ctx.place("core.wood", 1 + ctx.rand(3), cx - 3 + i, cy + 5)
