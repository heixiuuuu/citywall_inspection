package imageutil

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func orientedJPEG(orientation byte) []byte {
	var encoded bytes.Buffer
	_ = jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 20, 30)), nil)
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0,
		0x12, 1, 3, 0, 1, 0, 0, 0, orientation, 0, 0, 0, 0, 0, 0, 0}
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(exif)+2))
	data := append([]byte{}, encoded.Bytes()[:2]...)
	data = append(data, segment...)
	data = append(data, exif...)
	return append(data, encoded.Bytes()[2:]...)
}

func TestPreviewOrientationAndOriginal(t *testing.T) {
	for _, orientation := range []byte{1, 6, 8} {
		data := orientedJPEG(orientation)
		original := append([]byte{}, data...)
		preview, err := Preview(data, "image/jpeg")
		if err != nil {
			t.Fatal(err)
		}
		wantW, wantH := 20, 30
		if orientation != 1 {
			wantW, wantH = 30, 20
		}
		w, h := preview.Width, preview.Height
		if w != wantW || h != wantH {
			t.Fatalf("orientation %d: %dx%d", orientation, w, h)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(preview.Data))
		if err != nil || format != "jpeg" || cfg.Width != w || cfg.Height != h {
			t.Fatal("invalid preview")
		}
		if !bytes.Equal(data, original) {
			t.Fatal("original mutated")
		}
	}
}

func TestDecodeRejectsInvalidAndMismatchedContent(t *testing.T) {
	for _, input := range []struct {
		data      []byte
		mediaType string
	}{
		{[]byte("not an image"), "image/jpeg"},
		{orientedJPEG(6), "image/png"},
		{orientedJPEG(6)[:50], "image/jpeg"},
	} {
		if _, err := Decode(input.data, input.mediaType); err != ErrInvalidImage {
			t.Fatalf("got %v", err)
		}
	}
}

func TestDecodeRejectsExcessivePixelCount(t *testing.T) {
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	data := encoded.Bytes()
	binary.BigEndian.PutUint32(data[16:20], 100_000)
	binary.BigEndian.PutUint32(data[20:24], 100_000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	if _, err := Decode(data, "image/png"); err != ErrInvalidImage {
		t.Fatalf("got %v", err)
	}
}
