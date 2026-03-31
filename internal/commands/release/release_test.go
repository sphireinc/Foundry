package releasecmd

import "testing"

func TestVersionPattern(t *testing.T) {
	valid := []string{"v1.3.2", "v10.0.1"}
	for _, item := range valid {
		if !versionPattern.MatchString(item) {
			t.Fatalf("expected valid version %q", item)
		}
	}
	invalid := []string{"1.3.2", "v1.3", "v1.3.2-beta", "vx.y.z"}
	for _, item := range invalid {
		if versionPattern.MatchString(item) {
			t.Fatalf("expected invalid version %q", item)
		}
	}
}
