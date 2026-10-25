"""Grain and tall crops (PLAY-016, ART spec §9.3): corn, wheat, brewgrain,
cotton. Owned by the ART-GRAINS agent. BUILDERS maps species -> fn(stage,
v); species absent here still use the legacy builders in plant.py.
"""
BUILDERS = {}
