"""按固定类别选择模型输出，避免同类别候选重复进入人工复核。"""
from dataclasses import dataclass, field
from threading import Lock

from .protocol import Detector, DetectOutput
from .yolo import CLASS_CODES, DetectorError


@dataclass
class ClassRoutedDetector:
    primary: Detector
    specialist: Detector
    specialist_classes: frozenset[str]
    _lock: object = field(default_factory=Lock, repr=False, compare=False)

    def __post_init__(self):
        if not self.specialist_classes or not self.specialist_classes <= CLASS_CODES:
            raise ValueError("specialist classes must be nonempty business category codes")

    def detect(self, image_bytes: bytes) -> DetectOutput:
        with self._lock:
            primary = self.primary.detect(image_bytes)
            specialist = self.specialist.detect(image_bytes)
            if (primary.image_width, primary.image_height) != (specialist.image_width, specialist.image_height):
                raise DetectorError("INFERENCE_FAILED", "detector image coordinate systems do not match")
            detections = [item for item in primary.detections if item.class_code not in self.specialist_classes]
            detections.extend(item for item in specialist.detections if item.class_code in self.specialist_classes)
            return DetectOutput(primary.image_width, primary.image_height, [
                item.model_copy(update={"detection_id": f"d{index:04d}"}) for index, item in enumerate(detections)
            ])
