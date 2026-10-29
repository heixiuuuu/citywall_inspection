//go:build integration

package observation

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"citywall/server/internal/config"
	"citywall/server/internal/infrastructure/storage"
	"gorm.io/gorm"
)

func TestFailedImageUploadLeavesNoFileOrMetadata(t *testing.T) {
	for _, table := range []string{"images", "observation_images"} {
		t.Run(table, func(t *testing.T) {
			s, db, observation := persistentObservation(t)
			directory := filepath.Join(t.TempDir(), "images")
			store, err := storage.New(config.StorageConfig{ImageDir: directory, MaxImageBytes: 1 << 20, AllowedMediaTypes: []string{"image/png"}})
			if err != nil {
				t.Fatal(err)
			}
			s.storage = store
			var content bytes.Buffer
			if err := png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 80, 60))); err != nil {
				t.Fatal(err)
			}
			var before int64
			if err := db.Model(&Image{}).Count(&before).Error; err != nil {
				t.Fatal(err)
			}
			failure := errors.New("image storage registration unavailable")
			fail := true
			callback := "test:fail_image_upload"
			if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
				if fail && tx.Statement.Table == table {
					tx.AddError(failure)
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Callback().Create().Remove(callback) })
			if _, err := s.AddImage(context.Background(), inspector(), observation.ID, "image/png", content.Bytes(), ImageRoleDetail, "retry-upload"); !errors.Is(err, failure) {
				t.Fatalf("err=%v", err)
			}
			var after int64
			if err := db.Model(&Image{}).Count(&after).Error; err != nil {
				t.Fatal(err)
			}
			files, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			links, err := s.repo.ListImages(context.Background(), observation.ID)
			if err != nil {
				t.Fatal(err)
			}
			if before != after || len(files) != 0 || len(links) != 1 {
				t.Fatalf("orphan upload: images %d -> %d, files=%d links=%d", before, after, len(files), len(links))
			}
			fail = false
			image, err := s.AddImage(context.Background(), inspector(), observation.ID, "image/png", content.Bytes(), ImageRoleDetail, "retry-upload")
			if err != nil {
				t.Fatal(err)
			}
			files, err = os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			links, err = s.repo.ListImages(context.Background(), observation.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 1 || files[0].Name() != image.StorageKey || len(links) != 2 {
				t.Fatal("retry did not commit exactly one image")
			}
		})
	}
}
