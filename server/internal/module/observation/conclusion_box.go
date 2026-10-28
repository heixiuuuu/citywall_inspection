package observation

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"

	"citywall/server/internal/infrastructure/imageutil"
)

func validateConclusionBox(raw string, width, height int) error {
	var box struct {
		X1 *float64 `json:"x1"`
		Y1 *float64 `json:"y1"`
		X2 *float64 `json:"x2"`
		Y2 *float64 `json:"y2"`
	}
	if json.Unmarshal([]byte(raw), &box) != nil || box.X1 == nil || box.Y1 == nil || box.X2 == nil || box.Y2 == nil {
		return fmt.Errorf("%w: human bounding box requires numeric x1, y1, x2 and y2", ErrSourceConstraint)
	}
	if width <= 0 || height <= 0 || *box.X1 < 0 || *box.Y1 < 0 || *box.X1 >= *box.X2 || *box.Y1 >= *box.Y2 || *box.X2 > float64(width) || *box.Y2 > float64(height) {
		return fmt.Errorf("%w: human bounding box must fit the oriented image dimensions", ErrSourceConstraint)
	}
	return nil
}

func (s *service) conclusionImageSize(ctx context.Context, imageID int64) (image.Point, error) {
	metadata, err := s.repo.GetImage(ctx, imageID)
	if err != nil {
		return image.Point{}, err
	}
	file, err := s.storage.Open(metadata.StorageKey)
	if err != nil {
		return image.Point{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return image.Point{}, err
	}
	decoded, err := imageutil.Decode(data, metadata.MediaType)
	if err != nil {
		return image.Point{}, err
	}
	return decoded.Bounds().Size(), nil
}
