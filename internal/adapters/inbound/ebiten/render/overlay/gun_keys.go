package overlay

import (
	"math"

	"rimworld-go/internal/application/game"
)

// Gun frame keys of the wave D guns (DEF-3), spelled out so a frame lookup
// allocates nothing: the sentry turret, the autocannon and the slug turret
// share the aim frames 0..15; the mortar's tube takes its own.
const (
	AutocannonKind = "autocannon"
	SlugTurretKind = "slug turret"
	MortarKind     = "mortar"
)

// gunSpec is a gun post's base sprite and its 16 gun frames.
type gunSpec struct {
	base  string
	frame *[TurretAimFrames]string
}

// gunOf is the base and gun frames of a gun post kind (ok false: not one).
func gunOf(kind string) (gunSpec, bool) {
	switch kind {
	case SentryTurretKind:
		return gunSpec{TurretBaseKey, &sentryGunKeys}, true
	case AutocannonKind:
		return gunSpec{"defense/autocannon_base", &autocannonGunKeys}, true
	case SlugTurretKind:
		return gunSpec{"defense/slug_turret_base", &slugGunKeys}, true
	case MortarKind:
		return gunSpec{"defense/mortar_base", &mortarTubeKeys}, true
	}
	return gunSpec{}, false
}

// aimOf is the gun frame b points at: a mortar with a forced target aims at
// it (frame 0 points south, counting clockwise on screen), else AimDir.
func aimOf(b *game.BuildingView) int {
	if b.Kind != MortarKind || !b.HasTarget {
		return aimFrame(b.AimDir)
	}
	dx, dy := float64(b.TargetX-b.X), float64(b.TargetY-b.Y)
	if dx == 0 && dy == 0 {
		return aimFrame(b.AimDir)
	}
	th := math.Atan2(-dx, dy) * 180 / math.Pi
	return aimFrame(int8(math.Round(th / turretStepDeg)))
}

var sentryGunKeys = [TurretAimFrames]string{
	"defense/sentry_turret_gun_00",
	"defense/sentry_turret_gun_01",
	"defense/sentry_turret_gun_02",
	"defense/sentry_turret_gun_03",
	"defense/sentry_turret_gun_04",
	"defense/sentry_turret_gun_05",
	"defense/sentry_turret_gun_06",
	"defense/sentry_turret_gun_07",
	"defense/sentry_turret_gun_08",
	"defense/sentry_turret_gun_09",
	"defense/sentry_turret_gun_10",
	"defense/sentry_turret_gun_11",
	"defense/sentry_turret_gun_12",
	"defense/sentry_turret_gun_13",
	"defense/sentry_turret_gun_14",
	"defense/sentry_turret_gun_15",
}

var autocannonGunKeys = [TurretAimFrames]string{
	"defense/autocannon_gun_00",
	"defense/autocannon_gun_01",
	"defense/autocannon_gun_02",
	"defense/autocannon_gun_03",
	"defense/autocannon_gun_04",
	"defense/autocannon_gun_05",
	"defense/autocannon_gun_06",
	"defense/autocannon_gun_07",
	"defense/autocannon_gun_08",
	"defense/autocannon_gun_09",
	"defense/autocannon_gun_10",
	"defense/autocannon_gun_11",
	"defense/autocannon_gun_12",
	"defense/autocannon_gun_13",
	"defense/autocannon_gun_14",
	"defense/autocannon_gun_15",
}

var slugGunKeys = [TurretAimFrames]string{
	"defense/slug_turret_gun_00",
	"defense/slug_turret_gun_01",
	"defense/slug_turret_gun_02",
	"defense/slug_turret_gun_03",
	"defense/slug_turret_gun_04",
	"defense/slug_turret_gun_05",
	"defense/slug_turret_gun_06",
	"defense/slug_turret_gun_07",
	"defense/slug_turret_gun_08",
	"defense/slug_turret_gun_09",
	"defense/slug_turret_gun_10",
	"defense/slug_turret_gun_11",
	"defense/slug_turret_gun_12",
	"defense/slug_turret_gun_13",
	"defense/slug_turret_gun_14",
	"defense/slug_turret_gun_15",
}

var mortarTubeKeys = [TurretAimFrames]string{
	"defense/mortar_tube_00",
	"defense/mortar_tube_01",
	"defense/mortar_tube_02",
	"defense/mortar_tube_03",
	"defense/mortar_tube_04",
	"defense/mortar_tube_05",
	"defense/mortar_tube_06",
	"defense/mortar_tube_07",
	"defense/mortar_tube_08",
	"defense/mortar_tube_09",
	"defense/mortar_tube_10",
	"defense/mortar_tube_11",
	"defense/mortar_tube_12",
	"defense/mortar_tube_13",
	"defense/mortar_tube_14",
	"defense/mortar_tube_15",
}
