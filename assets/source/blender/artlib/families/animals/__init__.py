"""Animal archetype builders (ART-011), one module per archetype family.

Each archetype module exposes:
    ARCHETYPES = ("quadruped", "primate")      # roster archetypes it builds
    def build(row, static=False) -> [paths]     # row: animal_roster.Species

load() imports the modules that exist (bird, reptile and insect are
optional until A2 lands them) and returns {archetype: build}.
"""
import importlib

MODULES = ("quadruped", "bird", "reptile", "insect")
BUILDERS = {}


def load():
    BUILDERS.clear()
    for name in MODULES:
        try:
            mod = importlib.import_module("." + name, __name__)
        except ModuleNotFoundError as e:
            if e.name == "%s.%s" % (__name__, name):
                continue
            raise
        for arch in getattr(mod, "ARCHETYPES", ()):
            BUILDERS[arch] = mod.build
    return BUILDERS
