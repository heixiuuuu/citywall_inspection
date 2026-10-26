package imageutil

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

var ErrInvalidImage = errors.New("image: invalid content, media type or pixel dimensions")

const MaxPixels int64 = 64_000_000

// Decode 保留原件，仅对派生展示应用 EXIF 方向；坐标系与识别端一致。
func Decode(data []byte, mediaType string) (image.Image, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width) > MaxPixels/int64(cfg.Height) {
		return nil, ErrInvalidImage
	}
	if map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[format] != mediaType {
		return nil, ErrInvalidImage
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, ErrInvalidImage
	}
	return img, nil
}

type PreviewImage struct {
	Data          []byte
	Width, Height int
	MediaType     string
}

// Preview 保留完整像素坐标；JPEG 照片使用 JPEG，透明格式使用 PNG。
func Preview(data []byte, mediaType string) (*PreviewImage, error) {
	img, err := Decode(data, mediaType)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if mediaType == "image/jpeg" {
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 92})
	} else {
		mediaType = "image/png"
		err = png.Encode(&out, img)
	}
	if err != nil {
		return nil, err
	}
	return &PreviewImage{Data: out.Bytes(), Width: img.Bounds().Dx(), Height: img.Bounds().Dy(), MediaType: mediaType}, nil
}
