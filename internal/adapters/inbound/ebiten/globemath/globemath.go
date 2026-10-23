// Package globemath is the pure geometry of the landing-site globe:
// quaternions, the orbit camera, the perspective projection, ray-sphere
// picking and the horizon test. It has no Ebitengine dependency, so the
// picking and camera rules are unit-tested headless.
//
// Conventions. World space is the planet's: the unit sphere, north +Z.
// View space has +X to the right, +Y up and +Z towards the viewer; the
// eye sits at (0, 0, Dist) looking down -Z, Dist measured in planet radii.
package globemath

import "math"

// Vec3 is a 3-vector.
type Vec3 = [3]float64

// Camera distance limits, in planet radii (NG-1 §2.7).
const (
	MinDist     = 1.25
	MaxDist     = 4.0
	DefaultDist = 2.9
)

// MaxLat is how far the camera centre may tilt towards a pole (radians).
const MaxLat = 89.0 * math.Pi / 180

// InertiaDecay is the share of the spin kept per frame after a drag
// (8 % decay per frame).
const InertiaDecay = 0.92

// Dot, Cross, Len and Norm are the usual vector helpers.
func Dot(a, b Vec3) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func Cross(a, b Vec3) Vec3 {
	return Vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

func Len(a Vec3) float64 { return math.Sqrt(Dot(a, a)) }

// Norm is a scaled to unit length (the zero vector stays zero).
func Norm(a Vec3) Vec3 {
	l := Len(a)
	if l == 0 {
		return a
	}
	return Vec3{a[0] / l, a[1] / l, a[2] / l}
}

// Quat is a rotation quaternion (W + Xi + Yj + Zk).
type Quat struct{ W, X, Y, Z float64 }

// Identity is the no-op rotation.
func Identity() Quat { return Quat{W: 1} }

// AxisAngle is the rotation by ang radians about axis (right-handed).
func AxisAngle(axis Vec3, ang float64) Quat {
	a := Norm(axis)
	s, c := math.Sincos(ang / 2)
	return Quat{c, a[0] * s, a[1] * s, a[2] * s}
}

// Mul is the rotation r followed by q (q * r).
func (q Quat) Mul(r Quat) Quat {
	return Quat{
		q.W*r.W - q.X*r.X - q.Y*r.Y - q.Z*r.Z,
		q.W*r.X + q.X*r.W + q.Y*r.Z - q.Z*r.Y,
		q.W*r.Y - q.X*r.Z + q.Y*r.W + q.Z*r.X,
		q.W*r.Z + q.X*r.Y - q.Y*r.X + q.Z*r.W,
	}
}

// Conj is the inverse rotation of a unit quaternion.
func (q Quat) Conj() Quat { return Quat{q.W, -q.X, -q.Y, -q.Z} }

// Normalize rescales q to unit length (identity for the zero quaternion).
func (q Quat) Normalize() Quat {
	n := math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
	if n == 0 {
		return Identity()
	}
	return Quat{q.W / n, q.X / n, q.Y / n, q.Z / n}
}

// Rotate applies q to v.
func (q Quat) Rotate(v Vec3) Vec3 {
	m := q.Matrix()
	return Vec3{
		m[0]*v[0] + m[1]*v[1] + m[2]*v[2],
		m[3]*v[0] + m[4]*v[1] + m[5]*v[2],
		m[6]*v[0] + m[7]*v[1] + m[8]*v[2],
	}
}

// Matrix is q as a row-major 3x3 rotation matrix.
func (q Quat) Matrix() [9]float64 {
	w, x, y, z := q.W, q.X, q.Y, q.Z
	return [9]float64{
		1 - 2*(y*y+z*z), 2 * (x*y - z*w), 2 * (x*z + y*w),
		2 * (x*y + z*w), 1 - 2*(x*x+z*z), 2 * (y*z - x*w),
		2 * (x*z - y*w), 2 * (y*z + x*w), 1 - 2*(x*x+y*y),
	}
}

// Viewport maps view space to the screen: the principal point (CX, CY)
// and the focal length F in pixels. The y axis flips (screen y is down).
type Viewport struct{ CX, CY, F float64 }

// FocalPerSize is the focal length per pixel of the area's short side
// (about a 45 degree field of view).
const FocalPerSize = 1.2

// NewViewport is the viewport for the area (x, y, w, h) with the globe
// panned by (panX, panY) pixels from the area's centre.
func NewViewport(x, y, w, h, panX, panY float64) Viewport {
	return Viewport{CX: x + w/2 + panX, CY: y + h/2 + panY, F: FocalPerSize * math.Min(w, h)}
}

// HorizonZ is the view-space z of the horizon circle: a sphere point is
// facing the eye when its z is above it.
func HorizonZ(dist float64) float64 { return 1 / dist }

// SilhouetteRadius is the on-screen radius, in pixels, of the sphere.
func SilhouetteRadius(vp Viewport, dist float64) float64 {
	if dist <= 1 {
		return math.Inf(1)
	}
	return vp.F / math.Sqrt(dist*dist-1)
}

// ProjectView projects a view-space point to the screen. depth is the
// distance in front of the eye (dist - z); it is <= 0 for points at or
// behind the eye.
func (vp Viewport) ProjectView(dist float64, v Vec3) (sx, sy, depth float64) {
	depth = dist - v[2]
	if depth < 1e-9 {
		depth = 1e-9
	}
	k := vp.F / depth
	return vp.CX + v[0]*k, vp.CY - v[1]*k, dist - v[2]
}

// RayDir is the view-space direction of the ray from the eye through the
// screen point (sx, sy).
func (vp Viewport) RayDir(sx, sy float64) Vec3 {
	return Norm(Vec3{(sx - vp.CX) / vp.F, -(sy - vp.CY) / vp.F, -1})
}

// RaySphere intersects the ray origin + t*dir (dir unit) with the sphere
// of the given radius at the origin; t is the nearer hit.
func RaySphere(origin, dir Vec3, radius float64) (t float64, ok bool) {
	b := Dot(origin, dir)
	c := Dot(origin, origin) - radius*radius
	disc := b*b - c
	if disc < 0 {
		return 0, false
	}
	t = -b - math.Sqrt(disc)
	if t < 0 {
		return 0, false
	}
	return t, true
}

// ScreenToSphere is the view-space point of the unit sphere under the
// screen point (sx, sy), when the pointer is over the globe.
func (vp Viewport) ScreenToSphere(dist, sx, sy float64) (Vec3, bool) {
	eye := Vec3{0, 0, dist}
	d := vp.RayDir(sx, sy)
	t, ok := RaySphere(eye, d, 1)
	if !ok {
		return Vec3{}, false
	}
	return Vec3{eye[0] + d[0]*t, eye[1] + d[1]*t, eye[2] + d[2]*t}, true
}

// BackFacing reports a sphere point with view-space z is hidden from the
// eye, with slack for the tile's own size (a tile whose centre is just
// past the horizon still shows its near edge).
func BackFacing(z, dist, slack float64) bool { return z < HorizonZ(dist)-slack }

// Camera orbits the planet. It looks at (Lat, Lon) with north up; Dist is
// the eye distance in radii; Pan shifts the globe on the screen.
type Camera struct {
	Lat, Lon   float64 // the point under the view centre, radians
	Dist       float64
	PanX, PanY float64 // pixels

	distGoal float64
	spinLon  float64 // inertia: radians per frame
	spinLat  float64
	fly      flight
}

type flight struct {
	on         bool
	n, of      int
	lat0, lon0 float64
	lat1, lon1 float64
}

// NewCamera looks at the equator and the prime meridian at the default
// distance.
func NewCamera() *Camera {
	return &Camera{Dist: DefaultDist, distGoal: DefaultDist}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// ClampDist limits a distance to [MinDist, MaxDist].
func ClampDist(d float64) float64 { return clamp(d, MinDist, MaxDist) }

// Orientation is the world-to-view rotation: it brings the centre point
// to +Z with north (world +Z) pointing up the screen.
func (c *Camera) Orientation() Quat {
	qLon := AxisAngle(Vec3{0, 0, 1}, -c.Lon)
	qLat := AxisAngle(Vec3{0, 1, 0}, c.Lat-math.Pi/2)
	qUp := AxisAngle(Vec3{0, 0, 1}, -math.Pi/2)
	return qUp.Mul(qLat).Mul(qLon).Normalize()
}

// Centre is the unit world vector under the view centre.
func (c *Camera) Centre() Vec3 {
	cl := math.Cos(c.Lat)
	return Vec3{cl * math.Cos(c.Lon), cl * math.Sin(c.Lon), math.Sin(c.Lat)}
}

// ToView rotates a world vector into view space.
func (c *Camera) ToView(p Vec3) Vec3 { return c.Orientation().Rotate(p) }

// ToWorld rotates a view-space vector back into world space.
func (c *Camera) ToWorld(v Vec3) Vec3 { return c.Orientation().Conj().Rotate(v) }

// Viewport is the camera's screen mapping for the area (x, y, w, h).
func (c *Camera) Viewport(x, y, w, h float64) Viewport {
	return NewViewport(x, y, w, h, c.PanX, c.PanY)
}

// Project is the screen position of a world point and its view-space z.
func (c *Camera) Project(vp Viewport, p Vec3) (sx, sy, z float64) {
	v := c.ToView(p)
	sx, sy, _ = vp.ProjectView(c.Dist, v)
	return sx, sy, v[2]
}

// Pick is the world-space unit vector under the screen point, when the
// pointer is over the globe.
func (c *Camera) Pick(vp Viewport, sx, sy float64) (Vec3, bool) {
	v, ok := vp.ScreenToSphere(c.Dist, sx, sy)
	if !ok {
		return Vec3{}, false
	}
	return c.ToWorld(v), true
}

// RadiansPerPixel is how far a drag of one pixel turns the globe at the
// view centre.
func (c *Camera) RadiansPerPixel(vp Viewport) float64 {
	return (c.Dist - 1) / vp.F
}

// Rotate turns the globe so the surface follows a drag of (dx, dy) pixels
// (dragging right moves the surface right). Longitude moves faster near the
// poles, so the horizontal turn is divided by cos(lat) (floored).
func (c *Camera) Rotate(dx, dy float64, vp Viewport) {
	k := c.RadiansPerPixel(vp)
	c.turn(-dx*k/math.Max(math.Cos(c.Lat), 0.25), dy*k)
}

// Look moves the view centre east by dLon and north by dLat radians
// (keyboard rotation); it cancels inertia and any flight.
func (c *Camera) Look(dLon, dLat float64) {
	c.StopSpin()
	c.turn(dLon, dLat)
}

// Fling sets the inertia to the last drag step (dx, dy pixels per frame).
// With reduced motion there is no inertia.
func (c *Camera) Fling(dx, dy float64, vp Viewport, reducedMotion bool) {
	if reducedMotion {
		c.spinLon, c.spinLat = 0, 0
		return
	}
	k := c.RadiansPerPixel(vp)
	c.spinLon = -dx * k / math.Max(math.Cos(c.Lat), 0.25)
	c.spinLat = dy * k
}

// Spin is the current inertia in radians per frame.
func (c *Camera) Spin() (dLon, dLat float64) { return c.spinLon, c.spinLat }

// StopSpin cancels inertia and any flight (the player took the globe).
func (c *Camera) StopSpin() {
	c.spinLon, c.spinLat = 0, 0
	c.fly.on = false
}

func (c *Camera) turn(dLon, dLat float64) {
	c.Lon = wrapLon(c.Lon + dLon)
	c.Lat = clamp(c.Lat+dLat, -MaxLat, MaxLat)
}

func wrapLon(l float64) float64 {
	l = math.Mod(l+math.Pi, 2*math.Pi)
	if l < 0 {
		l += 2 * math.Pi
	}
	return l - math.Pi
}

// Zoom multiplies the distance goal by factor (<1 zooms in), clamped.
func (c *Camera) Zoom(factor float64) { c.distGoal = ClampDist(c.distGoal * factor) }

// SetDist sets the distance (and its goal) at once, clamped.
func (c *Camera) SetDist(d float64) {
	c.Dist = ClampDist(d)
	c.distGoal = c.Dist
}

// DistGoal is the distance the camera is easing to.
func (c *Camera) DistGoal() float64 { return c.distGoal }

// CentreOn looks at the world direction dir. frames <= 0 jumps; otherwise
// the camera flies there, easing in and out, over that many frames.
func (c *Camera) CentreOn(dir Vec3, frames int) {
	d := Norm(dir)
	if d == (Vec3{}) {
		return
	}
	lat := math.Asin(clamp(d[2], -1, 1))
	lon := c.Lon
	if math.Hypot(d[0], d[1]) > 1e-9 {
		lon = math.Atan2(d[1], d[0])
	}
	c.spinLon, c.spinLat = 0, 0
	if frames <= 0 {
		c.Lat, c.Lon = clamp(lat, -MaxLat, MaxLat), lon
		c.fly.on = false
		return
	}
	// Shortest way round in longitude.
	dl := wrapLon(lon - c.Lon)
	c.fly = flight{on: true, of: frames, lat0: c.Lat, lon0: c.Lon,
		lat1: clamp(lat, -MaxLat, MaxLat), lon1: c.Lon + dl}
}

// Flying reports a CentreOn flight is under way.
func (c *Camera) Flying() bool { return c.fly.on }

// Step advances one frame: the flight, the inertia and the zoom easing.
func (c *Camera) Step(reducedMotion bool) {
	if c.fly.on {
		c.fly.n++
		t := float64(c.fly.n) / float64(c.fly.of)
		if t >= 1 {
			t = 1
			c.fly.on = false
		}
		e := t * t * (3 - 2*t)
		c.Lat = c.fly.lat0 + (c.fly.lat1-c.fly.lat0)*e
		c.Lon = wrapLon(c.fly.lon0 + (c.fly.lon1-c.fly.lon0)*e)
	}
	if c.spinLon != 0 || c.spinLat != 0 {
		c.turn(c.spinLon, c.spinLat)
		c.spinLon *= InertiaDecay
		c.spinLat *= InertiaDecay
		if math.Abs(c.spinLon) < 1e-6 && math.Abs(c.spinLat) < 1e-6 {
			c.spinLon, c.spinLat = 0, 0
		}
	}
	if reducedMotion {
		c.Dist = c.distGoal
	} else if d := c.distGoal - c.Dist; math.Abs(d) < 1e-4 {
		c.Dist = c.distGoal
	} else {
		c.Dist += d * 0.3
	}
	c.Dist = ClampDist(c.Dist)
}

// ClampPan keeps the globe's centre within the area (w, h) of the screen.
func (c *Camera) ClampPan(w, h float64) {
	c.PanX = clamp(c.PanX, -w/2, w/2)
	c.PanY = clamp(c.PanY, -h/2, h/2)
}
