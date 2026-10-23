package globemath

import (
	"math"
	"math/rand"
	"testing"
)

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func vnear(a, b Vec3, eps float64) bool {
	return near(a[0], b[0], eps) && near(a[1], b[1], eps) && near(a[2], b[2], eps)
}

func TestQuatRotateAndConj(t *testing.T) {
	t.Parallel()
	q := AxisAngle(Vec3{0, 0, 1}, math.Pi/2)
	if got := q.Rotate(Vec3{1, 0, 0}); !vnear(got, Vec3{0, 1, 0}, 1e-12) {
		t.Fatalf("rotate x by 90 about z = %v", got)
	}
	r := AxisAngle(Vec3{1, 2, 3}, 0.7).Mul(q)
	v := Vec3{0.3, -0.5, 0.8}
	if got := r.Conj().Rotate(r.Rotate(v)); !vnear(got, v, 1e-12) {
		t.Fatalf("conj does not invert: %v", got)
	}
	if got := Identity().Normalize().Rotate(v); !vnear(got, v, 1e-12) {
		t.Fatalf("identity moved %v", got)
	}
	if got := (Quat{}).Normalize(); got != Identity() {
		t.Fatalf("zero normalises to %v", got)
	}
}

func TestOrientationCentresAndNorthIsUp(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ lat, lon float64 }{{0, 0}, {0.5, 2}, {-1.2, -2.5}, {1.5, 3}} {
		cam := NewCamera()
		cam.Lat, cam.Lon = c.lat, c.lon
		if got := cam.ToView(cam.Centre()); !vnear(got, Vec3{0, 0, 1}, 1e-12) {
			t.Fatalf("centre of %v maps to %v", c, got)
		}
		north := cam.ToView(Vec3{0, 0, 1})
		if north[1] < 0 || !near(north[0], 0, 1e-12) {
			t.Fatalf("north not up for %v: %v", c, north)
		}
		// East of the centre is to the right.
		east := cam.ToView(Norm(Vec3{-math.Sin(c.lon), math.Cos(c.lon), 0}))
		if east[0] <= 0 {
			t.Fatalf("east not right for %v: %v", c, east)
		}
	}
}

// TestPickRoundTrip projects many surface points through random cameras
// and picks them back off the screen.
func TestPickRoundTrip(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(7))
	checked := 0
	for i := 0; i < 400; i++ {
		cam := NewCamera()
		cam.Lat = (rng.Float64()*2 - 1) * MaxLat
		cam.Lon = (rng.Float64()*2 - 1) * math.Pi
		cam.SetDist(MinDist + rng.Float64()*(MaxDist-MinDist))
		cam.PanX, cam.PanY = (rng.Float64()-0.5)*300, (rng.Float64()-0.5)*200
		vp := cam.Viewport(0, 0, 1280, 720)
		for j := 0; j < 25; j++ {
			p := Norm(Vec3{rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()})
			sx, sy, z := cam.Project(vp, p)
			if BackFacing(z, cam.Dist, 0) || z < HorizonZ(cam.Dist)+0.02 {
				continue
			}
			got, ok := cam.Pick(vp, sx, sy)
			if !ok {
				t.Fatalf("front point %v not picked (cam %+v)", p, cam)
			}
			if !vnear(got, p, 1e-8) {
				t.Fatalf("picked %v, want %v", got, p)
			}
			checked++
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d points exercised", checked)
	}
}

func TestPickMissesOutsideTheLimb(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	vp := cam.Viewport(0, 0, 1280, 720)
	r := SilhouetteRadius(vp, cam.Dist)
	if _, ok := cam.Pick(vp, vp.CX+r*1.02, vp.CY); ok {
		t.Fatal("picked outside the silhouette")
	}
	if _, ok := cam.Pick(vp, vp.CX+r*0.98, vp.CY); !ok {
		t.Fatal("missed inside the silhouette")
	}
}

func TestSilhouetteMatchesLimbProjection(t *testing.T) {
	t.Parallel()
	vp := NewViewport(0, 0, 1000, 800, 0, 0)
	for _, d := range []float64{1.25, 2, 4} {
		zh := HorizonZ(d)
		v := Vec3{math.Sqrt(1 - zh*zh), 0, zh}
		sx, _, _ := vp.ProjectView(d, v)
		if !near(sx-vp.CX, SilhouetteRadius(vp, d), 1e-6) {
			t.Fatalf("d=%v limb at %v, silhouette %v", d, sx-vp.CX, SilhouetteRadius(vp, d))
		}
	}
}

func TestBackFaceCull(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	front := cam.Centre()
	back := Vec3{-front[0], -front[1], -front[2]}
	if _, _, z := cam.Project(cam.Viewport(0, 0, 800, 600), front); BackFacing(z, cam.Dist, 0.08) {
		t.Fatal("the centre is culled")
	}
	if _, _, z := cam.Project(cam.Viewport(0, 0, 800, 600), back); !BackFacing(z, cam.Dist, 0.08) {
		t.Fatal("the far side is drawn")
	}
	zh := HorizonZ(cam.Dist)
	if BackFacing(zh-0.05, cam.Dist, 0.08) || !BackFacing(zh-0.09, cam.Dist, 0.08) {
		t.Fatal("slack not applied around the horizon")
	}
}

func TestZoomClamp(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	for i := 0; i < 200; i++ {
		cam.Zoom(0.8)
		cam.Step(false)
	}
	if cam.Dist < MinDist-1e-9 || !near(cam.Dist, MinDist, 1e-3) {
		t.Fatalf("zoom in settles at %v, want %v", cam.Dist, MinDist)
	}
	for i := 0; i < 200; i++ {
		cam.Zoom(1.3)
		cam.Step(false)
	}
	if cam.Dist > MaxDist+1e-9 || !near(cam.Dist, MaxDist, 1e-3) {
		t.Fatalf("zoom out settles at %v, want %v", cam.Dist, MaxDist)
	}
	cam.SetDist(99)
	if cam.Dist != MaxDist || cam.DistGoal() != MaxDist {
		t.Fatalf("SetDist(99) = %v / %v", cam.Dist, cam.DistGoal())
	}
	cam.SetDist(0.1)
	if cam.Dist != MinDist {
		t.Fatalf("SetDist(0.1) = %v", cam.Dist)
	}
	cam.Zoom(0.5)
	cam.Step(true)
	if cam.Dist != MinDist {
		t.Fatalf("reduced motion zoom = %v", cam.Dist)
	}
}

func TestInertiaDecays(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	vp := cam.Viewport(0, 0, 1280, 720)
	cam.Fling(30, 0, vp, false)
	l0, _ := cam.Spin()
	cam.Step(false)
	l1, _ := cam.Spin()
	if !near(l1/l0, InertiaDecay, 1e-9) {
		t.Fatalf("decay ratio %v, want %v", l1/l0, InertiaDecay)
	}
	moved := cam.Lon
	for i := 0; i < 400; i++ {
		cam.Step(false)
	}
	if l, _ := cam.Spin(); l != 0 {
		t.Fatalf("still spinning: %v", l)
	}
	if cam.Lon == moved {
		t.Fatal("inertia did not turn the globe")
	}
	// Reduced motion: a fling does nothing.
	cam2 := NewCamera()
	cam2.Fling(30, 10, vp, true)
	if l, b := cam2.Spin(); l != 0 || b != 0 {
		t.Fatalf("reduced motion spins: %v %v", l, b)
	}
	cam.Fling(30, 0, vp, false)
	cam.StopSpin()
	if l, _ := cam.Spin(); l != 0 {
		t.Fatal("StopSpin left a spin")
	}
}

func TestDragFollowsTheSurface(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	vp := cam.Viewport(0, 0, 1280, 720)
	// Grab the point under the centre, drag 40 px right: it ends 40 px right
	// (to first order).
	p, _ := cam.Pick(vp, vp.CX, vp.CY)
	cam.Rotate(40, 0, vp)
	sx, sy, _ := cam.Project(vp, p)
	if !near(sy, vp.CY, 1e-6) || !near(sx-vp.CX, 40, 1.5) {
		t.Fatalf("grabbed point at (%v,%v), want ~(%v,%v)", sx, sy, vp.CX+40, vp.CY)
	}
	cam.Rotate(0, 5000, vp)
	if cam.Lat > MaxLat+1e-12 {
		t.Fatalf("latitude %v past the pole limit", cam.Lat)
	}
}

func TestCentreOnJumpsAndFliesTheShortWay(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	cam.Lon = 3.0
	target := Vec3{math.Cos(-3.0) * math.Cos(0.3), math.Sin(-3.0) * math.Cos(0.3), math.Sin(0.3)}
	cam.CentreOn(target, 20)
	if !cam.Flying() {
		t.Fatal("no flight")
	}
	prev := math.Abs(wrapLon(cam.Lon - (-3.0)))
	for i := 0; i < 30; i++ {
		cam.Step(false)
		d := math.Abs(wrapLon(cam.Lon - (-3.0)))
		if d > prev+1e-9 {
			t.Fatalf("flight went the long way at step %d", i)
		}
		prev = d
	}
	if cam.Flying() || !near(cam.Lat, 0.3, 1e-9) || !near(wrapLon(cam.Lon+3.0), 0, 1e-9) {
		t.Fatalf("ended at lat %v lon %v flying %v", cam.Lat, cam.Lon, cam.Flying())
	}
	cam.CentreOn(Vec3{1, 0, 0}, 0)
	if cam.Flying() || !near(cam.Lat, 0, 1e-12) || !near(cam.Lon, 0, 1e-12) {
		t.Fatalf("jump ended at %v %v", cam.Lat, cam.Lon)
	}
	cam.CentreOn(Vec3{}, 5) // the zero vector is ignored
	if cam.Flying() {
		t.Fatal("zero direction started a flight")
	}
}

func TestClampPan(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	cam.PanX, cam.PanY = 5000, -5000
	cam.ClampPan(800, 600)
	if cam.PanX != 400 || cam.PanY != -300 {
		t.Fatalf("pan %v,%v", cam.PanX, cam.PanY)
	}
}

func TestRaySphere(t *testing.T) {
	t.Parallel()
	if tt, ok := RaySphere(Vec3{0, 0, 3}, Vec3{0, 0, -1}, 1); !ok || !near(tt, 2, 1e-12) {
		t.Fatalf("head-on hit %v %v", tt, ok)
	}
	if _, ok := RaySphere(Vec3{0, 0, 3}, Vec3{0, 1, 0}, 1); ok {
		t.Fatal("perpendicular ray hit")
	}
	if _, ok := RaySphere(Vec3{0, 0, 3}, Vec3{0, 0, 1}, 1); ok {
		t.Fatal("ray pointing away hit")
	}
}

func TestLookMovesAndCancelsInertia(t *testing.T) {
	t.Parallel()
	cam := NewCamera()
	cam.Fling(20, 5, cam.Viewport(0, 0, 800, 600), false)
	cam.Look(0.1, 0.2)
	if l, b := cam.Spin(); l != 0 || b != 0 {
		t.Fatal("Look kept the inertia")
	}
	if !near(cam.Lon, 0.1, 1e-12) || !near(cam.Lat, 0.2, 1e-12) {
		t.Fatalf("looked to %v %v", cam.Lat, cam.Lon)
	}
	cam.Look(0, 10)
	if cam.Lat != MaxLat {
		t.Fatalf("latitude %v not clamped", cam.Lat)
	}
}
