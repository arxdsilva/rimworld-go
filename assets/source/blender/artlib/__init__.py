"""rimworld-go art library: the single source of truth for camera, light,
shading, post-processing and export of world sprites (ART spec §3-§7, §19-§25).

Every family module builds geometry with the helpers here and calls
stage.render_asset(); nothing else may set up a camera or light.
"""
