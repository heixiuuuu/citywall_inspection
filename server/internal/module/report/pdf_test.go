package report

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderTaskPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 64, 48))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	src := &TaskSource{
		TaskID: 1, Title: "测试任务", AssigneeName: "巡检员", Status: "ASSIGNED",
		PointTotal: 1,
		Points: []PointView{
			{TaskPointID: 1, SequenceNo: 1, PointName: "东城墙起点", PointCode: "E-001",
				Observation: &ObservationView{
					ID: 1, Status: "ACCEPTED", Outcome: "SUSPECTED", LocationText: "测试位置",
					Images:      []ImageView{{ID: 10, StorageKey: "none.png", MediaType: "image/png", Width: 64, Height: 48, Role: "OVERVIEW"}},
					Conclusions: []ConclusionView{{Items: []ItemView{{ImageID: 10, Decision: "ADD", FinalClassCode: "SURFACE_SPALLING", FinalBoxJSON: `{"x1":10,"y1":10,"x2":50,"y2":40}`}}}},
				}},
		},
	}
	opts := renderOpts{
		GeneratedBy: "user#1", SnapshotAt: "2026-10-07 12:00:00",
		FontPath: os.Getenv("CITY_WALL_TEST_FONT"),
		OpenImage: func(key string) (string, int, int, string, func(), error) {
			return path, 64, 48, "image/png", func() {}, nil
		},
	}
	data, err := renderTaskPDF(src, opts)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("pdf too small: %d", len(data))
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("bad pdf head: %q", data[:16])
	}
}
