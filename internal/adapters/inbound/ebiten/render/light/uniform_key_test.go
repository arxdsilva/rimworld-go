package light

import "testing"

// TestSyncUniformsFollowsLightChange: the uniforms are copied once per
// light and again when the light changes (GAR-154).
func TestSyncUniformsFollowsLightChange(t *testing.T) {
	var l Lit
	l.Light = LightAt(12, 100, "clear", 0)
	l.SyncUniforms()
	l.Uniforms[4][0] = -1 // a stale mark: an unchanged light must not rewrite it
	l.SyncUniforms()
	if l.Uniforms[4][0] != -1 {
		t.Fatal("copied again although the light is the same")
	}
	l.Light = LightAt(2, 100, "clear", 0)
	l.SyncUniforms()
	if l.Uniforms[4][0] != l.Light.SkyAmbient[0] || l.Uniforms[7][0] != l.Light.LampSky {
		t.Fatal("did not follow the new light")
	}
}
