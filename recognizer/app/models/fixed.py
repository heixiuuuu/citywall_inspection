"""固定响应检测器：用于验证契约，不接真实模型。

对应 03 文档“先用固定响应验证契约，再接真实 YOLO”的首条原型策略。
现在会解析图片并返回真实尺寸（EXIF 方向已归一），detections 保持可控。
"""
from __future__ import annotations

from .protocol import DetectOutput, Detector
from ..schemas import Detection
from ..images import inspect_image


class FixedDetector:
    """固定检测器：返回配置的候选（默认空），尺寸取真实图片解析值。

    detections=[] 表示模型未检出，不替人员填写“正常”。
    联调时可构造带候选的实例验证框坐标与人工确认链路。
    """

    def __init__(
        self,
        detections: list[Detection] | None = None,
    ) -> None:
        self._detections = detections or []

    def detect(self, image_bytes: bytes) -> DetectOutput:
        info = inspect_image(image_bytes, allowed_formats=[])
        return DetectOutput(
            image_width=info.width,
            image_height=info.height,
            detections=self._detections,
        )


def is_detector(obj: object) -> bool:
    """供测试与配置校验：确认对象实现了 Detector 协议的关键方法。"""
    return callable(getattr(obj, "detect", None))
