"""Quadruped and primate species (ART-011): one Body spec per roster key.

Numbers are in body lengths (rig.py scales the model to the roster size).
Each spec follows its roster `features` text; signature traits are
exaggerated so the species reads at gameplay zoom.
"""
from .. import animal_roster as R
from . import rig

ARCHETYPES = ("quadruped", "primate")
OUTLINE_PX = {}   # PLAY-022: XS and S are 1.3-1.5 tiles long now, so every class keeps the contracts outline

COW_PATCHES = [(0.2, (25, 140), 0.075, 0.06), (0.42, (70, 160), 0.085, 0.07), (0.66, (15, 110), 0.07, 0.06),
               (0.82, (60, 200), 0.06, 0.05), (0.32, (-20, 205), 0.05, 0.04)]

SPECS = {
    "deer": dict(
        height=0.62, torso=0.56, chest=(0.085, 0.125), waist=(0.075, 0.1), hips=(0.085, 0.112), rump_z=0.01,
        leg_r=(0.052, 0.032, 0.02, 0.016), hock=0.86, muscle=0.85, neck_len=0.25, neck_angle=60, neck_r=(0.055, 0.032),
        head=(0.04, 0.052, 0.042), head_angle=42, muzzle=(0.085, 0.021, 0.022), muzzle_drop=8,
        ears=("pointed", 1.15, 20), antlers=("deer", 1.0), tail=("short", 1.0, 25),
        pal=dict(coat="fur_deer", belly="fur_cream", throat="fur_cream", tail="fur_white", nose="eye_black",
                 hoof="fur_dark", antler="horn", ear_in="fur_cream", muzzle="fur_deer"),
        marks=(("belly", "fur_cream"), ("throat", "fur_cream"), ("rump", "fur_cream"))),
    "timber_wolf": dict(
        height=0.5, torso=0.6, chest=(0.08, 0.125), waist=(0.065, 0.08), hips=(0.075, 0.095), rump_z=-0.01,
        leg_r=(0.054, 0.034, 0.022, 0.019), foot="paw", hock=0.84, muscle=1.3, neck_len=0.15, neck_angle=32,
        neck_r=(0.085, 0.062), head=(0.06, 0.064, 0.054), head_angle=12, muzzle=(0.1, 0.027, 0.025),
        muzzle_drop=6, ears=("pointed", 1.1, 0), tail=("bushy", 1.05, -5), fangs=True,
        pal=dict(coat="fur_wolf", belly="fur_wolf_light", muzzle="fur_wolf_light", throat="fur_wolf_light",
                 dark="fur_dark", nose="eye_black", tail="fur_wolf", tail_tip="fur_dark", ear="fur_dark"),
        marks=(("belly", "fur_wolf_light"), ("throat", "fur_wolf_light"), ("saddle", "fur_dark", 0.55, 0.78, 0.15),
               ("neck_top", "fur_dark"))),
    "cow": dict(
        height=0.56, torso=0.7, chest=(0.14, 0.165), waist=(0.15, 0.165), hips=(0.135, 0.155), belly=0.035,
        torso_p=3.4, leg_r=(0.06, 0.042, 0.028, 0.025), hock=0.9, neck_len=0.13, neck_angle=22,
        neck_r=(0.11, 0.08), neck_y=0.05, head=(0.068, 0.07, 0.064), head_angle=42,
        muzzle=(0.1, 0.058, 0.052), muzzle_drop=6, ears=("side", 1.35, 0), horns=("cow", 1.5),
        tail=("tufted", 1.15, 0),
        pal=dict(coat="fur_white", nose="skin_pink", hoof="fur_black", tail_tip="fur_black", horn="horn_ivory",
                 ear_in="skin_pink", ear="fur_black", head="fur_white"),
        marks=(("patches", "fur_black", COW_PATCHES), ("eye_patch", "fur_black"))),
    "elephant": dict(
        height=0.64, torso=0.64, chest=(0.17, 0.19), waist=(0.17, 0.19), hips=(0.16, 0.18), rump_z=-0.025,
        arch=0.012, torso_p=2.3, leg_r=(0.085, 0.072, 0.064, 0.062), hock=0.985, hind_r=1.0, foot="pad",
        stance=0.6, neck_len=0.07, neck_angle=15, neck_r=(0.15, 0.13), neck_y=0.05,
        head=(0.105, 0.1, 0.115), head_angle=8, muzzle=(0.06, 0.06, 0.06), muzzle_drop=55,
        ears=("fan", 1.15, 0), tail=("tufted", 0.95, -5), eye=0.009,
        pal=dict(coat="skin_grey", hoof="horn_ivory", tail_tip="fur_dark", nose=None),
        marks=(("trunk", "skin_grey"), ("tusks", "horn_ivory", "elephant", 1.0)),
        poses=None),
    "woolhorn": dict(
        height=0.58, torso=0.64, chest=(0.15, 0.17), waist=(0.15, 0.17), hips=(0.14, 0.16), torso_p=3.0,
        leg_r=(0.06, 0.045, 0.032, 0.028), hock=0.92, neck_len=0.15, neck_angle=12, neck_r=(0.12, 0.09),
        neck_y=0.04, head=(0.075, 0.064, 0.064), head_angle=48, muzzle=(0.07, 0.058, 0.046), muzzle_drop=4,
        ears=("side", 0.9, 10), horns=("ram", 1.45), tail=("stub", 1.0, 0), eye=0.008,
        pal=dict(coat="wool_slate", nose="fur_dark", hoof="fur_black", horn="horn_dark", muzzle="wool_slate",
                 ear="wool_slate"),
        marks=(("skirt", "wool_slate", 0.075), ("ruff", "wool_slate", 1.0), ("blaze", "fur_cream", 0.8))),
    "squirrel": dict(
        height=0.34, torso=0.52, chest=(0.075, 0.095), waist=(0.075, 0.095), hips=(0.095, 0.115), rump_z=0.035,
        arch=0.02, torso_p=2.2, leg_r=(0.05, 0.03, 0.022, 0.02), foot="paw", hock=0.72, neck_len=0.07,
        neck_angle=50, neck_r=(0.075, 0.068), head=(0.085, 0.08, 0.076), head_angle=18,
        muzzle=(0.045, 0.036, 0.032), muzzle_drop=18, ears=("round", 1.0, 0), tail=("squirrel", 1.0, 0),
        pal=dict(coat="fur_squirrel", belly="fur_cream", nose="eye_black"),
        marks=(("belly", "fur_cream"),)),
}

# Shared builds; species override what sets them apart.
CANID = dict(
    height=0.5, torso=0.6, chest=(0.08, 0.125), waist=(0.065, 0.08), hips=(0.075, 0.095), rump_z=-0.01,
    leg_r=(0.054, 0.034, 0.022, 0.019), foot="paw", hock=0.84, muscle=1.3, neck_len=0.15, neck_angle=32,
    neck_r=(0.085, 0.062), head=(0.06, 0.064, 0.054), head_angle=12, muzzle=(0.1, 0.027, 0.025), muzzle_drop=6,
    ears=("pointed", 1.1, 0), tail=("bushy", 1.05, -5), fangs=True)
FOX = dict(CANID, height=0.4, torso=0.56, chest=(0.07, 0.1), waist=(0.06, 0.078), hips=(0.068, 0.088),
           leg_r=(0.042, 0.027, 0.017, 0.015), muscle=1.1, neck_len=0.12, neck_angle=35, neck_r=(0.065, 0.05),
           head=(0.052, 0.056, 0.046), head_angle=10, muzzle=(0.088, 0.02, 0.019), muzzle_drop=5,
           ears=("pointed", 1.0, 0), tail=("fox", 1.3, 0))
FELID = dict(
    height=0.42, torso=0.64, chest=(0.075, 0.11), waist=(0.065, 0.085), hips=(0.075, 0.1), rump_z=0.005,
    leg_r=(0.055, 0.035, 0.022, 0.02), foot="paw", hock=0.8, muscle=1.35, neck_len=0.12, neck_angle=30,
    neck_r=(0.07, 0.055), head=(0.062, 0.058, 0.055), head_angle=10, muzzle=(0.04, 0.03, 0.026), muzzle_drop=12,
    ears=("pointed", 0.8, 0), tail=("cat", 1.0, 0), fangs=True)
BEAR = dict(
    height=0.55, torso=0.62, chest=(0.15, 0.17), waist=(0.14, 0.16), hips=(0.14, 0.16), rump_z=-0.03,
    torso_p=2.4, leg_r=(0.07, 0.055, 0.04, 0.036), foot="claw", hock=0.97, muscle=1.2, hind_r=1.05,
    neck_len=0.1, neck_angle=15, neck_r=(0.12, 0.1), head=(0.075, 0.07, 0.065), head_angle=15,
    muzzle=(0.07, 0.035, 0.032), muzzle_drop=8, ears=("round", 0.85, 0), tail=("stub", 1.0, 0), fangs=True)
RAT = dict(
    height=0.26, torso=0.6, chest=(0.075, 0.08), waist=(0.075, 0.085), hips=(0.085, 0.09), rump_z=0.02,
    torso_p=2.2, leg_r=(0.035, 0.025, 0.017, 0.015), foot="paw", hock=0.8, neck_len=0.06, neck_angle=25,
    neck_r=(0.065, 0.05), head=(0.05, 0.06, 0.045), head_angle=10, muzzle=(0.08, 0.025, 0.022), muzzle_drop=10,
    ears=("round", 0.9, 0), tail=("thin", 1.0, 0))
HARE = dict(
    height=0.33, torso=0.52, chest=(0.08, 0.1), waist=(0.085, 0.105), hips=(0.115, 0.135), rump_z=0.05,
    arch=0.02, torso_p=2.2, leg_r=(0.04, 0.026, 0.018, 0.016), foot="paw", hock=0.62, hind_r=1.45, neck_len=0.06,
    neck_angle=45, neck_r=(0.06, 0.05), head=(0.05, 0.065, 0.05), head_angle=25, muzzle=(0.04, 0.03, 0.028),
    ears=("long", 1.0, 0), tail=("stub", 1.0, 20))
CERVID = dict(
    height=0.6, torso=0.6, chest=(0.1, 0.14), waist=(0.09, 0.115), hips=(0.095, 0.125),
    leg_r=(0.056, 0.035, 0.022, 0.019), muscle=0.92, hock=0.87, neck_len=0.25, neck_angle=48,
    neck_r=(0.08, 0.047), head=(0.046, 0.064, 0.049), head_angle=45, muzzle=(0.095, 0.027, 0.028),
    ears=("pointed", 1.0, 10), tail=("short", 1.0, 15))

SPECS.update({
    "alpaca": dict(
        height=0.48, torso=0.48, chest=(0.1, 0.12), waist=(0.1, 0.12), hips=(0.1, 0.12), torso_p=2.8,
        leg_r=(0.045, 0.03, 0.02, 0.018), hock=0.9, neck_len=0.38, neck_angle=84, neck_r=(0.052, 0.042),
        neck_y=0.08, head=(0.04, 0.05, 0.042), head_angle=18, muzzle=(0.045, 0.026, 0.026), muzzle_drop=10,
        ears=("pointed", 1.25, 0), tail=("short", 1.2, 10),
        pal=dict(coat="wool_cream", nose="fur_dark", hoof="fur_dark"),
        marks=(("wool", "wool_cream", 1.0, 0.34), ("neck_wool", "wool_cream", 1.0))),
    "bison": dict(
        height=0.62, torso=0.7, chest=(0.15, 0.2), waist=(0.12, 0.15), hips=(0.1, 0.12), rump_z=-0.12,
        torso_p=2.4, leg_r=(0.06, 0.045, 0.03, 0.026), hock=0.9, neck_len=0.1, neck_angle=-12,
        neck_r=(0.13, 0.1), neck_y=0.04, head=(0.072, 0.07, 0.072), head_angle=72, muzzle=(0.06, 0.05, 0.045),
        muzzle_drop=10, ears=("side", 0.8, 0), horns=("short_curved", 1.0), tail=("tufted", 0.8, 0),
        pal=dict(coat="fur_tawny", head="fur_choc", neck="fur_choc", muzzle="fur_choc", legs="fur_brown",
                 horn="horn_dark", hoof="fur_black", tail_tip="fur_choc", ear="fur_choc"),
        marks=(("hump", "fur_choc", 0.8, 0.9, 0.11, 0.2), ("shag", "fur_choc", 1.1), ("beard", "fur_choc", 1.4))),
    "emberbuck": dict(CERVID,
        height=0.58, torso=0.56, chest=(0.085, 0.12), waist=(0.075, 0.095), hips=(0.085, 0.11),
        leg_r=(0.05, 0.031, 0.019, 0.016), muscle=0.9, neck_len=0.22, neck_angle=55, neck_r=(0.06, 0.035),
        head=(0.042, 0.055, 0.044), head_angle=38, muzzle=(0.075, 0.022, 0.022), horns=("swept", 1.0),
        pal=dict(coat="fur_ochre", belly="fur_cream", tail="fur_cream", nose="eye_black", horn="horn_dark"),
        marks=(("belly", "fur_cream"), ("glands", "gland_ember", 7, 0.021, 0.15, 0.9),
               ("throat_gland", "gland_ember", 0.03))),
    "capybara": dict(
        height=0.36, torso=0.74, chest=(0.13, 0.14), waist=(0.13, 0.145), hips=(0.13, 0.14), torso_p=3.4,
        leg_r=(0.05, 0.04, 0.03, 0.028), foot="paw", hock=0.95, muscle=0.9, neck_len=0.06, neck_angle=20,
        neck_r=(0.12, 0.11), neck_y=0.04, head=(0.07, 0.075, 0.07), head_angle=15, muzzle=(0.1, 0.065, 0.065),
        muzzle_drop=8, ears=("tiny", 1.0, 0), tail=None, eye=0.008,
        pal=dict(coat="fur_chestnut", nose="fur_dark")),
    "caribou": dict(CERVID,
        antlers=("caribou", 1.15), ears=("pointed", 0.9, 10),
        pal=dict(coat="fur_brown", belly="fur_cream", throat="fur_cream", tail="fur_cream", nose="fur_dark"),
        marks=(("belly", "fur_cream"), ("throat", "fur_cream"), ("rump", "fur_cream", 1.2),
               ("ruff", "fur_cream", 0.85))),
    "donkey": dict(
        height=0.58, torso=0.56, chest=(0.1, 0.13), waist=(0.095, 0.12), hips=(0.1, 0.125),
        leg_r=(0.05, 0.034, 0.022, 0.019), hock=0.9, neck_len=0.2, neck_angle=48, neck_r=(0.08, 0.055),
        head=(0.05, 0.07, 0.055), head_angle=45, muzzle=(0.11, 0.04, 0.04), muzzle_drop=6,
        ears=("long", 1.0, 0), tail=("tufted", 0.9, 0),
        pal=dict(coat="fur_taupe", muzzle="fur_cream", belly="fur_ash", nose="fur_dark", tail_tip="fur_dark",
                 ear_tip="fur_dark"),
        marks=(("mane", "fur_dark", 0.03, 0.012), ("belly", "fur_ash"))),
    "dromedary": dict(
        height=0.72, torso=0.55, chest=(0.1, 0.13), waist=(0.09, 0.12), hips=(0.09, 0.115),
        leg_r=(0.05, 0.032, 0.022, 0.022), hock=0.88, foot="pad", neck_len=0.36, neck_angle=14, neck_bow=-0.32,
        neck_r=(0.07, 0.04), head=(0.04, 0.06, 0.045), head_angle=22, muzzle=(0.08, 0.025, 0.028), muzzle_drop=18,
        ears=("round", 0.6, 0), tail=("tufted", 0.6, 0),
        pal=dict(coat="fur_sand", hoof="fur_tan", tail_tip="fur_tan", nose="fur_tan"),
        marks=(("hump", "fur_sand", 0.5, 0.8, 0.15, 0.17),)),
    "elk": dict(CERVID,
        # heavy cervid: ~1.5x the deer's girth, deep chest, thick dark maned neck
        height=0.73, torso=0.62, chest=(0.13, 0.185), waist=(0.115, 0.15), hips=(0.12, 0.155), rump_z=-0.01,
        leg_r=(0.075, 0.046, 0.028, 0.024), muscle=1.15, hock=0.88, neck_len=0.28, neck_angle=50,
        neck_r=(0.125, 0.07), head=(0.056, 0.076, 0.058), head_angle=48, muzzle=(0.1, 0.034, 0.036),
        antlers=("elk", 1.15),
        pal=dict(coat="fur_tan", neck="fur_choc", head="fur_brown", muzzle="fur_brown", legs="fur_brown",
                 tail="fur_cream", nose="fur_dark"),
        marks=(("rump", "fur_cream", 1.25), ("ruff", "fur_choc", 0.72), ("throat", "fur_choc", -1, 0.4),
               ("mane", "fur_choc", 0.03, 0.03))),
    "gazelle": dict(CERVID,
        height=0.6, torso=0.52, chest=(0.075, 0.105), waist=(0.065, 0.08), hips=(0.075, 0.095),
        leg_r=(0.042, 0.026, 0.016, 0.013), muscle=0.8, hock=0.86, neck_len=0.23, neck_angle=55,
        neck_r=(0.05, 0.03), head=(0.036, 0.05, 0.038), head_angle=40, muzzle=(0.08, 0.018, 0.019),
        ears=("pointed", 1.1, 15), horns=("swept", 1.45, True),
        pal=dict(coat="fur_tan", belly="fur_white", tail="fur_white", horn="horn_dark", nose="eye_black"),
        marks=(("belly", "fur_white"), ("stripe", "fur_choc"), ("rump", "fur_white"))),
    "goat": dict(
        height=0.5, torso=0.58, chest=(0.09, 0.12), waist=(0.09, 0.115), hips=(0.09, 0.115), torso_p=3.2,
        leg_r=(0.045, 0.03, 0.02, 0.017), hock=0.88, neck_len=0.17, neck_angle=55, neck_r=(0.065, 0.04),
        head=(0.045, 0.06, 0.048), head_angle=45, muzzle=(0.075, 0.026, 0.028), ears=("side", 0.9, 0),
        horns=("swept", 1.2), tail=("short", 1.0, 55),
        pal=dict(coat="wool_cream", horn="horn_dark", nose="fur_dark", hoof="fur_dark"),
        marks=(("patches", "fur_tan", [(0.3, (40, 150), 0.07, 0.05), (0.7, (110, 20), 0.06, 0.05)]),
               ("beard", "fur_tan", 1.0))),
    "horse": dict(
        height=0.64, torso=0.6, chest=(0.1, 0.15), waist=(0.095, 0.125), hips=(0.1, 0.13),
        leg_r=(0.058, 0.036, 0.022, 0.019), muscle=1.1, hock=0.88, neck_len=0.3, neck_angle=55, neck_bow=0.12,
        neck_r=(0.085, 0.05), head=(0.045, 0.06, 0.05), head_angle=55, muzzle=(0.13, 0.035, 0.036),
        muzzle_drop=4, ears=("pointed", 0.8, 0), tail=("flowing", 1.0, 0),
        pal=dict(coat="fur_chestnut", tail="fur_choc", nose="fur_dark", hoof="fur_dark"),
        marks=(("mane", "fur_choc", 0.045, 0.014), ("blaze", "fur_white", 0.35))),
    "ibex": dict(
        height=0.52, torso=0.56, chest=(0.095, 0.125), waist=(0.09, 0.115), hips=(0.09, 0.115),
        leg_r=(0.048, 0.032, 0.021, 0.018), hock=0.88, neck_len=0.16, neck_angle=50, neck_r=(0.075, 0.05),
        head=(0.048, 0.06, 0.05), head_angle=45, muzzle=(0.07, 0.027, 0.028), ears=("side", 0.7, 0),
        horns=("sweep_big", 1.3, True), tail=("short", 0.9, 30),
        pal=dict(coat="fur_taupe", belly="fur_cream", horn="horn_dark", nose="fur_dark"),
        marks=(("belly", "fur_cream"), ("beard", "fur_brown", 1.1))),
    "ground_sloth": dict(
        height=0.5, torso=0.66, chest=(0.17, 0.2), waist=(0.16, 0.18), hips=(0.15, 0.17), rump_z=-0.05,
        torso_p=2.4, leg_r=(0.08, 0.06, 0.045, 0.04), foot="bigclaw", hock=0.95, muscle=1.2, hind_r=1.0,
        neck_len=0.1, neck_angle=12, neck_r=(0.13, 0.1), head=(0.06, 0.065, 0.055), head_angle=25,
        muzzle=(0.05, 0.04, 0.035), ears=("tiny", 1.0, 0), tail=("stub", 1.6, 0), eye=0.006,
        pal=dict(coat="fur_taupe", hoof="horn_ivory", nose="fur_dark"),
        marks=(("skirt", "fur_taupe", 0.15, 0.0, 1.12, 30, 0.06), ("ruff", "fur_taupe", 1.25))),
    "pig": dict(
        height=0.4, torso=0.68, chest=(0.12, 0.13), waist=(0.125, 0.14), hips=(0.12, 0.13), belly=0.02,
        torso_p=3.0, leg_r=(0.045, 0.035, 0.025, 0.022), hock=0.92, neck_len=0.06, neck_angle=10,
        neck_r=(0.11, 0.09), neck_y=0.04, head=(0.065, 0.06, 0.06), head_angle=15, muzzle=(0.08, 0.04, 0.038),
        muzzle_drop=0, ears=("floppy", 0.9, 0), tail=("curly", 1.0, 0),
        pal=dict(coat="skin_pink", nose="skin_rose", hoof="skin_rose")),
    "rhinoceros": dict(
        height=0.55, torso=0.66, chest=(0.16, 0.18), waist=(0.16, 0.18), hips=(0.15, 0.17), torso_p=2.8,
        leg_r=(0.075, 0.065, 0.055, 0.052), foot="pad", hock=0.97, hind_r=1.05, neck_len=0.08, neck_angle=0,
        neck_r=(0.13, 0.11), neck_y=0.04, head=(0.08, 0.09, 0.07), head_angle=30, muzzle=(0.12, 0.06, 0.055),
        muzzle_drop=10, ears=("pointed", 0.8, 0), tail=("tufted", 0.7, 0), eye=0.006,
        pal=dict(coat="skin_slate", hoof="skin_grey", tail_tip="fur_dark", nose=None), nod=0.35,
        marks=(("nose_horns", "horn", 1.1), ("folds", "skin_slate", (0.78, 0.25), 1.05))),
    "sheep": dict(
        height=0.45, torso=0.55, chest=(0.1, 0.12), waist=(0.1, 0.12), hips=(0.1, 0.12), torso_p=2.8,
        leg_r=(0.035, 0.025, 0.017, 0.015), hock=0.9, neck_len=0.1, neck_angle=40, neck_r=(0.06, 0.045),
        head=(0.045, 0.06, 0.045), head_angle=40, muzzle=(0.06, 0.028, 0.028), ears=("side", 0.8, 0),
        tail=("short", 1.0, 0),
        pal=dict(coat="wool_cream", head="fur_dark", muzzle="fur_dark", ear="fur_dark", legs="fur_dark",
                 nose="fur_black", hoof="fur_black"),
        marks=(("wool", "wool_cream", 1.15, 0.36), ("neck_wool", "wool_cream", 1.0))),
    "yak": dict(
        height=0.58, torso=0.68, chest=(0.14, 0.17), waist=(0.13, 0.16), hips=(0.13, 0.15), rump_z=-0.04,
        leg_r=(0.06, 0.045, 0.03, 0.026), hock=0.92, neck_len=0.1, neck_angle=5, neck_r=(0.12, 0.1),
        neck_y=0.04, head=(0.06, 0.07, 0.06), head_angle=55, muzzle=(0.07, 0.045, 0.04),
        ears=("side", 0.8, 0), horns=("out", 1.25), tail=("flowing", 0.7, 0),
        pal=dict(coat="fur_choc", horn="horn_ivory", nose="fur_black", hoof="fur_black", tail="fur_choc"),
        marks=(("hump", "fur_choc", 0.82, 0.85, 0.07, 0.16), ("skirt", "fur_choc", 0.06, -0.02, 1.1, 30, 0.05))),
    "timbergnaw": dict(
        height=0.34, torso=0.66, chest=(0.13, 0.13), waist=(0.14, 0.14), hips=(0.14, 0.14), rump_z=0.02,
        torso_p=2.3, leg_r=(0.045, 0.035, 0.026, 0.024), foot="paw", hock=0.9, neck_len=0.06, neck_angle=20,
        neck_r=(0.11, 0.095), head=(0.07, 0.075, 0.065), head_angle=15, muzzle=(0.06, 0.05, 0.045),
        muzzle_drop=10, ears=("round", 0.7, 0), tail=("paddle", 1.35, 0),
        pal=dict(coat="fur_chestnut", tail_tip="fur_choc", nose="fur_dark"),
        marks=(("incisors", "tooth_orange"),)),
    "arctic_fox": dict(FOX, pal=dict(coat="fur_white", nose="eye_black")),
    "arctic_wolf": dict(CANID, tail=("bushy", 1.15, -5),
        pal=dict(coat="fur_white", nose="eye_black"), marks=(("ruff", "fur_white", 1.15),)),
    "cinder_rat": dict(RAT,
        pal=dict(coat="fur_dark", tail="fur_dark", tail_tip="gland_ember", nose="skin_pink"),
        marks=(("throat_gland", "gland_ember", 0.04, 2.3, "jaw"),)),
    "cat": dict(FELID, ears=("pointed", 0.85, 0),
        pal=dict(coat="fur_ginger", belly="fur_cream", muzzle="fur_cream", ear_in="skin_pink", nose="skin_rose"),
        marks=(("belly", "fur_cream"), ("bands", "fur_rust"))),
    "chinchilla": dict(
        height=0.4, torso=0.48, chest=(0.14, 0.15), waist=(0.15, 0.16), hips=(0.15, 0.16), torso_p=2.0,
        leg_r=(0.035, 0.03, 0.024, 0.022), foot="paw", hock=0.9, neck_len=0.03, neck_angle=30,
        neck_r=(0.13, 0.12), head=(0.09, 0.085, 0.085), head_angle=10, muzzle=(0.03, 0.04, 0.035),
        muzzle_drop=15, ears=("round", 2.2, 0), tail=("bushy", 0.7, 30),
        pal=dict(coat="fur_ash", belly="fur_white", ear_in="skin_pink", nose="skin_rose"),
        marks=(("belly", "fur_white"),)),
    "cougar": dict(FELID, torso=0.68, ears=("round", 0.75, 0), tail=("cat", 1.25, -10),
        pal=dict(coat="fur_tawny", belly="fur_cream", muzzle="fur_cream", tail_tip="fur_choc", nose="skin_rose"),
        marks=(("belly", "fur_cream"),)),
    "fennec_fox": dict(FOX, ears=("huge", 1.0, 0), tail=("fox", 1.1, 0),
        pal=dict(coat="fur_sand", ear_in="fur_cream", tail_tip="fur_choc", nose="eye_black", belly="fur_cream"),
        marks=(("belly", "fur_cream"),)),
    "grizzly_bear": dict(BEAR,
        pal=dict(coat="fur_brown", muzzle="fur_tawny", hoof="fur_dark", nose="fur_black"),
        marks=(("hump", "fur_tawny", 0.82, 0.85, 0.075, 0.15),)),
    "guinea_pig": dict(
        height=0.3, torso=0.7, chest=(0.13, 0.13), waist=(0.14, 0.14), hips=(0.14, 0.14), torso_p=2.2,
        leg_r=(0.035, 0.03, 0.024, 0.022), foot="paw", hock=0.9, neck_len=0.03, neck_angle=15,
        neck_r=(0.13, 0.12), head=(0.09, 0.09, 0.08), head_angle=10, muzzle=(0.04, 0.045, 0.04),
        muzzle_drop=10, ears=("round", 0.8, 0), tail=None,
        pal=dict(coat="fur_white", nose="skin_rose", ear="fur_ginger"),
        marks=(("patches", "fur_ginger", [(0.25, (60, 150), 0.09, 0.07), (0.62, (100, 30), 0.08, 0.07)]),
               ("patches", "fur_brown", [(0.45, (10, 175), 0.06, 0.05)]), ("eye_patch", "fur_ginger"))),
    "hare": dict(HARE, ears=("long", 1.5, 5), pal=dict(coat="fur_wolf", belly="fur_cream", tail="fur_white", nose="skin_rose"),
                 marks=(("belly", "fur_cream"),)),
    "husky": dict(CANID, height=0.48, tail=("curled", 1.0, 0),
        pal=dict(coat="fur_white", head="fur_dark", ear="fur_dark", tail="fur_dark", tail_tip="fur_white",
                 nose="eye_black"),
        marks=(("saddle", "fur_dark", 0.7, 0.85, 0.1), ("neck_top", "fur_dark"), ("ruff", "fur_white", 1.0))),
    "labrador": dict(CANID, height=0.47, chest=(0.088, 0.13), waist=(0.078, 0.095), hips=(0.082, 0.1),
        head=(0.066, 0.066, 0.058), muzzle=(0.08, 0.032, 0.03), ears=("floppy", 1.35, 0), tail=("taper", 1.0, 0),
        muscle=1.2, pal=dict(coat="fur_gold", nose="fur_dark")),
    "lynx": dict(FELID, height=0.5, torso=0.55, leg_r=(0.05, 0.034, 0.024, 0.026),
        head=(0.068, 0.06, 0.06), muzzle=(0.035, 0.03, 0.026), ears=("tufted", 1.0, 0), tail=("bob", 1.0, 10),
        pal=dict(coat="fur_taupe", tail_tip="fur_black", ear_tip="fur_black", belly="fur_cream", nose="skin_rose"),
        marks=(("cheeks", "fur_ash", 1.0), ("spots", "fur_brown", 14, 0.012), ("belly", "fur_cream"))),
    "monkey": dict(
        height=0.4, torso=0.45, chest=(0.085, 0.1), waist=(0.07, 0.085), hips=(0.08, 0.095), rump_z=0.04,
        torso_p=2.2, leg_r=(0.035, 0.026, 0.018, 0.017), foot="paw", hock=0.9, neck_len=0.06, neck_angle=40,
        neck_r=(0.055, 0.05), head=(0.075, 0.07, 0.072), head_angle=0, muzzle=(0.025, 0.035, 0.03),
        muzzle_drop=10, ears=("round", 1.0, 0), tail=("arc", 1.0, 0),
        pal=dict(coat="fur_brown", muzzle="fur_sand", ear_in="skin_pink", nose="fur_dark"),
        marks=(("face_disc", "fur_sand"),)),
    "panther": dict(FELID, ears=("round", 0.75, 0), tail=("cat", 1.25, -10),
        pal=dict(coat="fur_black", eye="crop_fruit", nose="fur_dark")),
    "polar_bear": dict(BEAR, neck_len=0.18, neck_angle=25, neck_r=(0.11, 0.08), muzzle=(0.1, 0.032, 0.03),
        ears=("round", 0.7, 0), pal=dict(coat="fur_ivory", hoof="fur_dark", nose="fur_black")),
    "raccoon": dict(
        height=0.38, torso=0.55, chest=(0.1, 0.11), waist=(0.105, 0.115), hips=(0.11, 0.12), rump_z=0.03,
        torso_p=2.4, leg_r=(0.04, 0.03, 0.02, 0.018), foot="paw", hock=0.85, neck_len=0.06, neck_angle=25,
        neck_r=(0.08, 0.07), head=(0.07, 0.065, 0.058), head_angle=10, muzzle=(0.06, 0.024, 0.022),
        muzzle_drop=8, ears=("round", 0.9, 0), tail=("ringed", 1.0, 0),
        pal=dict(coat="fur_wolf", head="fur_ash", muzzle="fur_white", ear_in="fur_white", nose="fur_black",
                 rings=("fur_wolf", "fur_black")),
        marks=(("mask", "fur_black"),)),
    "rat": dict(RAT, tail=("thin", 1.15, 0),
        pal=dict(coat="fur_wolf", ear="skin_pink", tail="skin_pink", nose="skin_pink")),
    "red_fox": dict(FOX,
        pal=dict(coat="fur_rust", muzzle="fur_white", throat="fur_white", belly="fur_white", lower="fur_black",
                 ear="fur_black", ear_in="fur_white", tail_tip="fur_white", nose="eye_black"),
        marks=(("belly", "fur_white"), ("throat", "fur_white"), ("chest", "fur_white"))),
    "snowhare": dict(HARE, ears=("long", 1.5, 5), pal=dict(coat="fur_white", ear_tip="fur_black", nose="skin_rose")),
    "dire_wolf": dict(CANID, height=0.52, chest=(0.1, 0.14), waist=(0.075, 0.09), hips=(0.085, 0.105), arch=0.02,
        neck_r=(0.1, 0.075), head=(0.075, 0.075, 0.065), muzzle=(0.11, 0.036, 0.033), muscle=1.4,
        ears=("pointed", 0.95, 0),
        pal=dict(coat="fur_dark", belly="fur_wolf", muzzle="fur_wolf", eye="crop_fruit", nose="fur_black"),
        marks=(("belly", "fur_wolf"), ("ruff", "fur_dark", 1.25), ("crest", "fur_dark", 7, 0.035, 0.02))),
    "wild_boar": dict(
        height=0.48, torso=0.62, chest=(0.11, 0.15), waist=(0.1, 0.12), hips=(0.09, 0.11), rump_z=-0.06,
        torso_p=2.6, leg_r=(0.045, 0.032, 0.02, 0.017), hock=0.9, neck_len=0.06, neck_angle=5,
        neck_r=(0.11, 0.08), head=(0.055, 0.075, 0.06), head_angle=25, muzzle=(0.13, 0.032, 0.03),
        muzzle_drop=8, ears=("pointed", 0.7, 0), tail=("tufted", 0.5, 0), fangs=False,
        pal=dict(coat="fur_boar", nose="fur_dark", tail_tip="fur_black", hoof="fur_black"),
        marks=(("crest", "fur_black", 10, 0.05, 0.022), ("tusks", "horn_ivory", "boar", 1.2),
               ("hump", "fur_boar", 0.85, 0.85, 0.05, 0.15))),
    "yorkshire_terrier": dict(
        height=0.38, torso=0.55, chest=(0.08, 0.1), waist=(0.075, 0.09), hips=(0.08, 0.095),
        leg_r=(0.035, 0.025, 0.018, 0.016), foot="paw", hock=0.85, neck_len=0.14, neck_angle=58,
        neck_r=(0.065, 0.055), head=(0.085, 0.078, 0.072), head_angle=10, muzzle=(0.045, 0.028, 0.025),
        ears=("pointed", 1.1, 0), tail=("short", 1.2, 40),
        pal=dict(coat="fur_steel", head="fur_gold", legs="fur_gold", muzzle="fur_gold", neck="fur_gold",
                 ear="fur_gold", nose="fur_black", hoof="fur_gold"),
        marks=(("skirt", "fur_steel", 0.07, 0.03, 1.15, 48, 0.03, 1.45, 6, 0.6), ("chest", "fur_gold"))),
    "glimmerstag": dict(CERVID,
        # giant and massive: broad deep chest, thick neck, heavy legs; antlers ~30% smaller than before
        height=0.76, torso=0.64, chest=(0.165, 0.21), waist=(0.14, 0.17), hips=(0.145, 0.175), rump_z=-0.02,
        leg_r=(0.085, 0.056, 0.034, 0.029), muscle=1.2, hock=0.88, neck_len=0.31, neck_angle=55, neck_bow=0.08,
        neck_r=(0.14, 0.072), head=(0.055, 0.075, 0.056), head_angle=42, muzzle=(0.1, 0.03, 0.032),
        antlers=("crystal", 0.95), eye=0.006,
        pal=dict(coat="fur_ivory", antler="crystal_blue", antler_tip="crystal_glow", hoof="fur_steel",
                 nose="fur_steel"),
        marks=(("neck_top", "fur_moss"), ("mane", "fur_moss", 0.06, 0.035), ("ruff", "fur_moss", 0.7),
               ("saddle", "fur_moss", 0.9, 0.82, 0.68), ("crest", "fur_moss", 6, 0.06, 0.04, 0.66, 0.98))),
})

# Dimorphic species: the female set overrides (ART §11.1).
FEMALE = {
    "deer": dict(antlers=None, neck_r=(0.052, 0.03)),
    "elk": dict(antlers=None),
}


def spec(row):
    kw = dict(SPECS[row.key])
    L = R.body_length(row.size)
    kw.setdefault("eye", min(0.03, max(0.0075, 0.0095 / L)))
    kw.setdefault("outline", OUTLINE_PX.get(row.size.rstrip("+-")))
    b = rig.Body(key=row.key, length=L, attack=row.attack, archetype=row.archetype, **kw)
    if row.dimorphic and row.key in FEMALE:
        b.female = rig.Body(**{**b.__dict__, **FEMALE[row.key], "female": None})
    return b


def build(row, static=False):
    if row.key not in SPECS:
        print("quadruped: no spec for %s yet; skipped" % row.key)
        return []
    return rig.render_species(spec(row), static=static)
