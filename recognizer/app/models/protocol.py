"""Detector 协议：识别服务与具体模型之间的统一接口。

便于用测试替身（FixedDetector）验证契约，再无缝切换到真实 YOLO。
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

from ..schemas import Detection


@dataclass
class DetectOutput:
    """检测输出：原图尺寸 + 原图坐标系中的候选列表。"""

    image_width: int
    image_height: int
    detections: list[Detection]


class Detector(Protocol):
    """任何实现该协议的适配器都可接入识别服务。"""

    def detect(self, image_bytes: bytes) -> DetectOutput:
        """输入原始图片字节，返回原图坐标系中的检测结果。"""
        ...
