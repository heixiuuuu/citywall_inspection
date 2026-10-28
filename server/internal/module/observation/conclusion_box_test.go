package observation

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

type conclusionImageStorage struct {
	fakeStorage
	path string
}

func (s *conclusionImageStorage) Open(string) (*os.File, error) { return os.Open(s.path) }

func TestConclusionBoxValidationDoesNotPersistInvalidItems(t *testing.T) {
	cases := []struct {
		name  string
		box   string
		valid bool
	}{
		{"omitted", "", true},
		{"full image", `{"x1":0,"y1":0,"x2":100,"y2":80}`, true},
		{"fractional coordinates", `{"x1":0.5,"y1":1.5,"x2":99.5,"y2":79.5}`, true},
		{"invalid JSON", `not JSON`, false},
		{"null", `null`, false},
		{"missing coordinate", `{"x1":0,"y1":0,"x2":10}`, false},
		{"null coordinate", `{"x1":null,"y1":0,"x2":10,"y2":10}`, false},
		{"string coordinate", `{"x1":"0","y1":0,"x2":10,"y2":10}`, false},
		{"negative coordinate", `{"x1":-1,"y1":0,"x2":10,"y2":10}`, false},
		{"empty box", `{"x1":10,"y1":0,"x2":10,"y2":10}`, false},
		{"reversed box", `{"x1":20,"y1":20,"x2":10,"y2":10}`, false},
		{"outside image", `{"x1":0,"y1":0,"x2":101,"y2":80}`, false},
		{"nonfinite coordinate", `{"x1":0,"y1":0,"x2":1e309,"y2":10}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, repo := newObsService()
			tp := int64(1)
			o, err := s.Create(ctx, inspector(), SourceTask, &tp, "loc")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "sample.png")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 100, 80))); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			s.storage = &conclusionImageStorage{path: path}
			photo := &Image{Width: 100, Height: 80, MediaType: "image/png"}
			if err := repo.InsertImage(ctx, photo); err != nil {
				t.Fatal(err)
			}
			if err := repo.LinkImage(ctx, o.ID, photo.ID, ImageRoleOverview); err != nil {
				t.Fatal(err)
			}
			input := ConclusionInput{Outcome: OutcomeSuspected, Items: []ConclusionItemIn{
				{ImageID: photo.ID, Decision: "ADD", FinalClassCode: "CRACK"},
				{ImageID: photo.ID, Decision: "ADD", FinalClassCode: "SURFACE_SPALLING", FinalBoxJSON: tc.box},
			}}
			_, err = s.SaveConclusion(ctx, inspector(), o.ID, 1, input)
			if tc.valid {
				if err != nil || len(repo.concl) != 1 {
					t.Fatalf("valid conclusion: %v", err)
				}
			} else {
				if !errors.Is(err, ErrSourceConstraint) {
					t.Fatalf("expected validation error, got %v", err)
				}
				if len(repo.concl) != 0 || len(repo.conclusionItems) != 0 || o.Revision != 1 {
					t.Fatal("invalid coordinates changed persisted conclusion or record revision")
				}
			}
		})
	}
}
