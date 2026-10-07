package report

import (
	"bytes"
	"os"
	"testing"
)

func TestRenderTaskPDF(t *testing.T) {
	src := &TaskSource{
		TaskID: 1, Title: "测试任务", AssigneeName: "巡检员", Status: "ASSIGNED",
		PointTotal: 1,
		Points: []PointView{
			{TaskPointID: 1, SequenceNo: 1, PointName: "东城墙起点", PointCode: "E-001",
				Observation: &ObservationView{
					ID: 1, Status: "ACCEPTED", Outcome: "SUSPECTED", LocationText: "测试位置",
					Images: []ImageView{{ID: 10, StorageKey: "none.png", MediaType: "image/png", Width: 64, Height: 48, Role: "OVERVIEW"}},
				}},
		},
	}
	opts := renderOpts{
		GeneratedBy: "user#1", SnapshotAt: "2026-10-07 12:00:00",
		FontPath: `C:\Windows\Fonts\simhei.ttf`,
		OpenImage: func(key string) (string, int, int, string, func(), error) {
			return "none.png", 64, 48, "image/png", func() {}, nil
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
	if err := os.WriteFile("render_test.pdf", data, 0o644); err != nil {
		t.Fatal(err)
	}
}
