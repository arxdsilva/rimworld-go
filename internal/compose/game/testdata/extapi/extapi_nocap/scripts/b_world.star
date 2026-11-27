def on_hour(ctx):
    ctx.message("nocap animals %d" % len(ctx.animals()))
