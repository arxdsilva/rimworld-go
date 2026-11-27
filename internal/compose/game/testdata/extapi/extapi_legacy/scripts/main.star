# A pre-API script: the legacy set (query, commands, events) is all it has.
EVENTS = ["research_done"]

def on_day(ctx):
    ctx.message("legacy day %d" % ctx.day)

def on_event(ctx, event):
    ctx.message("legacy saw " + event.kind)
