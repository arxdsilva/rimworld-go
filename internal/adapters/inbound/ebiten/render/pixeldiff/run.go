package pixeldiff

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// Result is what Run writes to report.json.
type Result struct {
	Policy        string  `json:"policy"`
	Deterministic bool    `json:"deterministic"` // A == A' byte for byte
	Pass          bool    `json:"pass"`
	Reason        string  `json:"reason"`
	Report        Report  `json:"report"`
	BaselineDrift *Report `json:"baseline_drift,omitempty"`
}

// Run renders the scene three times through render (variant 0 is the old
// path, 1 the new one): old to A, new to B, old again to A'. It requires
// A == A', compares A with B under pol, and writes A.png, B.png, heat.png
// and report.json into dir. The returned error is non-nil when the scene is
// not deterministic or the policy fails.
func Run(dir string, pol Policy, render func(variant int) *image.RGBA) (Result, error) {
	a, b, a2 := render(0), render(1), render(0)
	res := Result{Policy: pol.Name}
	drift := Compare(a, a2, nil)
	res.Deterministic = !drift.SizeMismatch && drift.Differing == 0
	if !res.Deterministic {
		res.BaselineDrift = &drift
	}
	res.Report = Compare(a, b, EdgeMask(a, 48))
	res.Pass, res.Reason = res.Report.Pass(pol)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}
	imgs := map[string]*image.RGBA{"A.png": a, "B.png": b, "heat.png": HeatMap(a, b)}
	if !res.Deterministic {
		imgs["drift.png"] = HeatMap(a, a2) // where A and A' disagree
	}
	for name, img := range imgs {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return res, err
		}
		err = png.Encode(f, img)
		f.Close()
		if err != nil {
			return res, err
		}
	}
	js, _ := json.MarshalIndent(res, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "report.json"), js, 0o644); err != nil {
		return res, err
	}
	switch {
	case !res.Deterministic:
		return res, fmt.Errorf("scene is not deterministic: A and A' differ in %d pixels (max %d) within %v", drift.Differing, drift.Max, drift.Bounds)
	case !res.Pass:
		return res, fmt.Errorf("pixel diff fails policy %s: %s", pol.Name, res.Reason)
	}
	return res, nil
}
