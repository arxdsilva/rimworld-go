# recipe_done needs the recipes capability, which this plugin does not declare.
EVENTS = ["recipe_done"]

def on_event(ctx, event):
    ctx.message("nocap event delivered")
