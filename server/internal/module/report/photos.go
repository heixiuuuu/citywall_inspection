package report

import (
	"fmt"
	"math"
)

func (w *pdfWriter) photos(images []ImageView, opts renderOpts) {
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
		w.section(fmt.Sprintf("照片 #%d（%s）", image.ID, image.Role))
		if err := w.image(path, imgFormat(mediaType), width, height, 120); err != nil {
			w.text(fmt.Sprintf("照片 #%d 无法嵌入", image.ID), 9, 0)
		}
		if cleanup != nil {
			cleanup()
		}
	}
}
