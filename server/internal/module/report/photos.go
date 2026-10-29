package report

import (
	"encoding/json"
	"fmt"
	"math"
)

type photoBox struct{ X1, Y1, X2, Y2 float64 }

func (w *pdfWriter) photos(images []ImageView, conclusions []ConclusionView, opts renderOpts) {
	if opts.OpenImage == nil {
		return
	}
	for _, image := range images {
		path, width, height, mediaType, cleanup, err := opts.OpenImage(image.StorageKey)
		if err != nil {
			w.text(fmt.Sprintf("照片 #%d 无法读取", image.ID), 9, 0)
			continue
		}
		photoHeight := 90.0
		if width > 0 && height > 0 {
			photoHeight = math.Min(120*float64(height)/float64(width), 90)
		}
		w.ensure(photoHeight + sectionGap + lineHeight + 8)
		role := map[string]string{"OVERVIEW": "概览", "DETAIL": "细节", "ARCHIVED": "历史提交"}[image.Role]
		if role == "" {
			role = image.Role
		}
		var boxes []photoBox
		if len(conclusions) > 0 {
			for _, item := range conclusions[len(conclusions)-1].Items {
				if item.ImageID != image.ID || item.Decision != "ADD" {
					continue
				}
				var box photoBox
				if json.Unmarshal([]byte(item.FinalBoxJSON), &box) == nil && box.X1 >= 0 && box.Y1 >= 0 && box.X2 > box.X1 && box.Y2 > box.Y1 && box.X2 <= float64(width) && box.Y2 <= float64(height) {
					boxes = append(boxes, box)
				}
			}
		}
		heading := fmt.Sprintf("照片 #%d（%s）", image.ID, role)
		if len(boxes) > 0 {
			heading += " · 红框为人工补录"
		}
		w.section(heading)
		if err := w.image(path, imgFormat(mediaType), width, height, 120, boxes); err != nil {
			w.text(fmt.Sprintf("照片 #%d 无法嵌入", image.ID), 9, 0)
		}
		if cleanup != nil {
			cleanup()
		}
	}
}
