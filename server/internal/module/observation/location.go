package observation

import (
	"context"
	"fmt"
	"math"
)

func validateLocation(latitude, longitude, accuracy *float64) error {
	if (latitude == nil) != (longitude == nil) {
		return fmt.Errorf("%w: latitude and longitude must be available together", ErrSourceConstraint)
	}
	for _, value := range []*float64{latitude, longitude, accuracy} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return fmt.Errorf("%w: location values must be finite", ErrSourceConstraint)
		}
	}
	if latitude != nil && (*latitude < -90 || *latitude > 90 || *longitude < -180 || *longitude > 180) {
		return fmt.Errorf("%w: coordinates are outside WGS84 ranges", ErrSourceConstraint)
	}
	if accuracy != nil && *accuracy < 0 {
		return fmt.Errorf("%w: location accuracy must not be negative", ErrSourceConstraint)
	}
	return nil
}

func distanceMeters(latitude, longitude, targetLatitude, targetLongitude float64) float64 {
	const radians = math.Pi / 180
	deltaLatitude, deltaLongitude := (targetLatitude-latitude)*radians, (targetLongitude-longitude)*radians
	a := math.Pow(math.Sin(deltaLatitude/2), 2) + math.Cos(latitude*radians)*math.Cos(targetLatitude*radians)*math.Pow(math.Sin(deltaLongitude/2), 2)
	a = math.Max(0, math.Min(1, a))
	return 2 * 6371000 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func (s *service) locationDistance(ctx context.Context, o *Observation, latitude, longitude *float64) (distance, radius *float64, err error) {
	if o.Source != SourceTask || o.TaskPointID == nil {
		return nil, nil, nil
	}
	point, err := s.taskPoints.GetTaskPoint(ctx, *o.TaskPointID)
	if err != nil {
		return nil, nil, err
	}
	radius = point.SnapshotRadiusM
	if latitude == nil || longitude == nil || point.SnapshotLatitude == nil || point.SnapshotLongitude == nil {
		return nil, radius, nil
	}
	value := math.Round(distanceMeters(*latitude, *longitude, *point.SnapshotLatitude, *point.SnapshotLongitude)*100) / 100
	return &value, radius, nil
}
