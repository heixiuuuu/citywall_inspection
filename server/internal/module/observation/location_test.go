package observation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestInvalidLocationPatchLeavesRecordUnchanged(t *testing.T) {
	value := func(n float64) *float64 { return &n }
	cases := []PatchInput{
		{Latitude: value(91), Longitude: value(0)},
		{Latitude: value(0), Longitude: value(181)},
		{Latitude: value(1)},
		{Longitude: value(1)},
		{Latitude: value(math.NaN()), Longitude: value(0)},
		{Latitude: value(0), Longitude: value(math.Inf(1))},
		{AccuracyM: value(-1)},
		{AccuracyM: value(math.NaN())},
	}
	for index, input := range cases {
		s, _ := newObsService()
		o, err := s.Create(context.Background(), inspector(), SourceVoluntary, nil, "original")
		if err != nil {
			t.Fatal(err)
		}
		input.LocationText = "must not persist"
		_, err = s.Update(context.Background(), inspector(), o.ID, 1, input)
		if !errors.Is(err, ErrSourceConstraint) {
			t.Fatalf("case %d: %v", index, err)
		}
		if o.LocationText != "original" || o.Latitude != nil || o.Longitude != nil || o.AccuracyM != nil || o.Revision != 1 {
			t.Fatalf("case %d changed original record", index)
		}
	}
}

func TestLocationSupportsZeroLimitsAndPartialUpdateOfExistingPair(t *testing.T) {
	s, _ := newObsService()
	ctx := context.Background()
	o, _ := s.Create(ctx, inspector(), SourceVoluntary, nil, "loc")
	zero, latitude, longitude := 0.0, 90.0, 180.0
	if _, err := s.Update(ctx, inspector(), o.ID, 1, PatchInput{Latitude: &latitude, Longitude: &longitude, AccuracyM: &zero}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, inspector(), o.ID, 2, PatchInput{Latitude: &zero}); err != nil {
		t.Fatal(err)
	}
	if *o.Latitude != 0 || *o.Longitude != 180 || *o.AccuracyM != 0 {
		t.Fatal("valid coordinates not preserved")
	}
}

func TestDistanceUsesDispatchedPointSnapshot(t *testing.T) {
	s, _ := newObsService()
	zero, radius, latitude, accuracy := 0.0, 100.0, 0.001, 45.0
	point := s.taskPoints.(*fakeTaskPoint).points[1]
	point.SnapshotLatitude, point.SnapshotLongitude, point.SnapshotRadiusM = &zero, &zero, &radius
	tp := int64(1)
	o, _ := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	updated, err := s.Update(context.Background(), inspector(), o.ID, 1, PatchInput{Latitude: &latitude, Longitude: &zero, AccuracyM: &accuracy})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DistanceM == nil || math.Abs(*updated.DistanceM-111.19) > .01 {
		t.Fatal("incorrect target distance")
	}
	if distanceMeters(0, 0, 0, 0) != 0 || math.Abs(distanceMeters(0, 179.999, 0, -179.999)-222.39) > .1 {
		t.Fatal("incorrect zero or dateline distance")
	}
}

func TestSubmitRejectsInvalidStoredCoordinates(t *testing.T) {
	s, repo := newObsService()
	ctx := context.Background()
	tp := int64(1)
	o, _ := s.Create(ctx, inspector(), SourceTask, &tp, "loc")
	_, _ = s.AddImage(ctx, inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview, "")
	_, _ = s.SaveConclusion(ctx, inspector(), o.ID, 1, ConclusionInput{Outcome: OutcomeNormal})
	latitude, longitude := 91.0, 0.0
	o.Latitude, o.Longitude = &latitude, &longitude
	if _, err := s.Submit(ctx, inspector(), o.ID, 2); !errors.Is(err, ErrSourceConstraint) {
		t.Fatal(err)
	}
	if len(repo.subs) != 0 || o.Status != ObsDraft || o.Revision != 2 {
		t.Fatal("invalid location was submitted")
	}
}
