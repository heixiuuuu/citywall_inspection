package report

import (
	"strings"
	"testing"
)

func TestLocationWarningUsesConfiguredRadiusAndKeepsAccuracyWarning(t *testing.T) {
	distance, radius, accuracy := 80.0, 100.0, 45.0
	o := Observation{DistanceM: &distance}
	if got := locationAbnormal(o, &radius); got != "" {
		t.Fatalf("inside configured radius: %s", got)
	}
	distance = 110
	o.AccuracyM = &accuracy
	got := locationAbnormal(o, &radius)
	if !strings.Contains(got, "提示半径") || !strings.Contains(got, "定位精度") {
		t.Fatal(got)
	}
	o.AccuracyM = nil
	if got := locationAbnormal(o, nil); got != "" {
		t.Fatalf("unknown radius must not become 50m: %s", got)
	}
}
