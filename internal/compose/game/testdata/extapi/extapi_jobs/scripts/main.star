def on_pawn(ctx, pawn):
    if ctx.order(pawn.id, "haul", 4, 6):
        ctx.message("ordered haul %d" % pawn.id)
