"""固定响应检测器：用于验证契约，不接真实模型。

对应 03 文档“先用固定响应验证契约，再接真实 YOLO”的首条原型策略。
"""
from __future__ import annotations

from .protocol import DetectOutput, Detector
from ..schemas import Detection


class FixedDetector:
    """返回空候选的固定检测器。

    detections=[] 表示模型未检出，不替人员填写“正常”。
    联调时可构造带候选的实例验证框坐标与人工确认链路。
    """

    def __init__(
        self,
        image_width: int = 1600,
        image_height: int = 1200,
        detections: list[Detection] | None = None,
    ) -> None:
        self._width = image_width
        self._height = image_height
        self._detections = detections or []

    def detect(self, image_bytes: bytes) -> DetectOutput:
        # TODO(init): 真实实现校验格式与尺寸；当前不解析图片。
        return DetectOutput(
            image_width=self._width,
            image_height=self._height,
            detections=self._detections,
        )


def is_detector(obj: object) -> bool:
    """供测试与配置校验：确认对象实现了 Detector 协议的关键方法。"""
    return callable(getattr(obj, "detect", None))
