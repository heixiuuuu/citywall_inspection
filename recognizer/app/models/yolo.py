"""YOLOv8 真实推理适配器（ultralytics）。

坐标系契约（03 文档第 7 节）：bbox 为「按 EXIF 方向统一后的原图像素坐标」。
流程：
  1. 懒加载 ultralytics 模型（首次 detect 时加载，权重缺失/损坏抛 MODEL_UNAVAILABLE）；
  2. 解码图片 → EXIF 方向归一（ImageOps.exif_transpose）→ RGB；
  3. 推理：ultralytics 内部做 letterbox，results[0].boxes.xyxy 已映射回传入图尺寸
     （即 EXIF 归一后的原图坐标系），因此与契约坐标系一致；
  4. 类别码映射：模型 int 类别 → 业务 class_code（未知标签直接跳过并计数，不静默转“正常”）；
  5. 置信度阈值过滤。

零检测返回空候选：不自动形成“正常”结论。
"""
from __future__ import annotations

import io
import logging
from dataclasses import dataclass, field
from pathlib import Path
from threading import Lock

from PIL import Image, ImageOps

from .protocol import DetectOutput, Detector
from ..schemas import BBox, Detection

logger = logging.getLogger(__name__)


class DetectorError(Exception):
    """模型加载或推理失败（对应契约 MODEL_UNAVAILABLE / INFERENCE_FAILED）。"""

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message


# data.yaml 的 10 类统一映射（与 Go 端 category_code 字典保持一致）。
DEFAULT_CLASS_NAMES: dict[int, str] = {
    0: "CRACK",
    1: "DISCOLOURATION",
    2: "EFFLORESCENCE",
    3: "SURFACE_DISSOLUTION",
    4: "MASONRY_LOSS",
    5: "FUNGUS",
    6: "MOSS",
    7: "PLANT",
    8: "CORROSION",
    9: "ARTISTIC",
}


def resolve_weights_path(weights_path: str) -> Path:
    """把配置里的权重路径解析为绝对路径。

    优先级：绝对路径 → 相对当前工作目录 → 相对仓库根（config 文件上级）
    → recognizer/models/（旧占位目录）。
    """
    p = Path(weights_path)
    if p.is_absolute():
        return p
    for base in (Path.cwd(), Path(__file__).resolve().parents[3], Path(__file__).resolve().parents[1] / "models"):
        cand = base / weights_path
        if cand.is_file():
            return cand
    return Path(weights_path)


@dataclass
class YOLODetector:
    """YOLOv8 检测适配器：懒加载模型 + 原图坐标输出。"""

    weights_path: str
    confidence_threshold: float = 0.35
    class_names: dict[int, str] = field(default_factory=lambda: dict(DEFAULT_CLASS_NAMES))
    _model: object = field(default=None, repr=False, compare=False)
    _lock: object = field(default_factory=Lock, repr=False, compare=False)

    def _load_model(self) -> object:
        if self._model is not None:
            return self._model
        resolved = resolve_weights_path(self.weights_path)
        if not resolved.is_file():
            raise DetectorError("MODEL_UNAVAILABLE", f"weights not found: {resolved}")
        try:
            from ultralytics import YOLO  # 延迟导入，避免拖慢服务启动

            model = YOLO(str(resolved))
            names = getattr(model, "names", {})
            if not names or any(self.class_names.get(int(index)) != name for index, name in names.items()):
                raise DetectorError("MODEL_UNAVAILABLE", "model category names do not match business category codes")
            logger.info("YOLO model loaded: %s", resolved)
            self._model = model
            return model
        except Exception as e:
            raise DetectorError("MODEL_UNAVAILABLE", f"failed to load model: {e}") from e

    def detect(self, image_bytes: bytes) -> DetectOutput:
        with self._lock:
            return self._detect(image_bytes)

    def _detect(self, image_bytes: bytes) -> DetectOutput:
        model = self._load_model()
        try:
            im = Image.open(io.BytesIO(image_bytes))
            im = ImageOps.exif_transpose(im) or im
            rgb = im.convert("RGB")
            results = model.predict(rgb, conf=self.confidence_threshold, verbose=False)
        except DetectorError:
            raise
        except Exception as e:
            raise DetectorError("INFERENCE_FAILED", f"inference failed: {e}") from e

        width, height = rgb.size
        detections: list[Detection] = []
        unknown_labels = 0
        if results is not None and len(results) > 0:
            boxes = results[0].boxes
            if boxes is not None and boxes.xyxy is not None:
                names = getattr(results[0], "names", None) or {}
                xyxy = boxes.xyxy.tolist() if hasattr(boxes.xyxy, "tolist") else boxes.xyxy
                cls = boxes.cls.tolist() if hasattr(boxes.cls, "tolist") else boxes.cls
                conf = boxes.conf.tolist() if hasattr(boxes.conf, "tolist") else boxes.conf
                for i, (x1, y1, x2, y2) in enumerate(xyxy):
                    cls_id = int(cls[i])
                    c_conf = float(conf[i])
                    if c_conf < self.confidence_threshold:
                        continue  # 适配器自身再按阈值过滤，与 predict(conf=) 幂等
                    code = self.class_names.get(cls_id)
                    if code is None:
                        unknown_labels += 1
                        continue  # 未知类别跳过，不静默转“正常”
                    ix1, iy1 = max(0, min(width, int(round(x1)))), max(0, min(height, int(round(y1))))
                    ix2, iy2 = max(0, min(width, int(round(x2)))), max(0, min(height, int(round(y2))))
                    if ix1 >= ix2 or iy1 >= iy2:
                        continue
                    detections.append(
                        Detection(
                            detection_id=f"d{i:04d}",
                            class_code=code,
                            confidence=round(c_conf, 4),
                            bbox=BBox(
                                x1=ix1, y1=iy1, x2=ix2, y2=iy2,
                            ),
                        )
                    )

        if unknown_labels:
            logger.warning("skipped %d detections with unknown class ids", unknown_labels)

        return DetectOutput(
            image_width=width,
            image_height=height,
            detections=detections,
        )
