"""局部检测结果合并到方向归一后的原图坐标。"""
import io
from dataclasses import dataclass

from PIL import Image, ImageOps

from .protocol import DetectOutput, Detector
from .yolo import DetectorError
from ..schemas import BBox


def positions(length: int, size: int) -> list[int]:
    if length <= size:
        return [0]
    step = max(1, size * 3 // 4)
    result = list(range(0, length - size + 1, step))
    if result[-1] != length - size:
        result.append(length - size)
    return result


def overlap(a: BBox, b: BBox) -> float:
    intersection = max(0, min(a.x2, b.x2) - max(a.x1, b.x1)) * max(0, min(a.y2, b.y2) - max(a.y1, b.y1))
    union = (a.x2 - a.x1) * (a.y2 - a.y1) + (b.x2 - b.x1) * (b.y2 - b.y1) - intersection
    return intersection / union if union else 0.0


@dataclass
class TiledDetector:
    detector: Detector
    tile_size: int

    def detect(self, image_bytes: bytes) -> DetectOutput:
        image = ImageOps.exif_transpose(Image.open(io.BytesIO(image_bytes))).convert("RGB")
        width, height = image.size
        if max(image.size) > self.tile_size * 3:
            image.thumbnail((self.tile_size * 3, self.tile_size * 3), Image.Resampling.LANCZOS)
        scale_x, scale_y = width / image.width, height / image.height
        candidates = []
        for top in positions(image.height, self.tile_size):
            for left in positions(image.width, self.tile_size):
                tile = image.crop((left, top, min(left + self.tile_size, image.width), min(top + self.tile_size, image.height)))
                stream = io.BytesIO()
                tile.save(stream, format="PNG")
                output = self.detector.detect(stream.getvalue())
                if (output.image_width, output.image_height) != tile.size:
                    raise DetectorError("INFERENCE_FAILED", "tile coordinate system does not match")
                for item in output.detections:
                    box = item.bbox
                    mapped = BBox(x1=round((left + box.x1) * scale_x), y1=round((top + box.y1) * scale_y),
                                  x2=round((left + box.x2) * scale_x), y2=round((top + box.y2) * scale_y))
                    candidates.append(item.model_copy(update={"bbox": mapped}))
        selected = []
        for item in sorted(candidates, key=lambda item: item.confidence, reverse=True):
            if not any(item.class_code == other.class_code and overlap(item.bbox, other.bbox) >= .5 for other in selected):
                selected.append(item.model_copy(update={"detection_id": f"d{len(selected):04d}"}))
        return DetectOutput(width, height, selected)
