"""YOLO 模型适配（占位）。

接入真实 YOLO 时（03 文档第 5 节）：
  - 模型启动时加载一次，权重版本、标签映射与阈值由部署配置记录；
  - 输出转换成 fixed 契约（原图坐标系 bbox + class_code + confidence）；
  - 标签 class_code 必须映射到经确认的病害类别字典，未知标签不能静默转成“正常”。
"""
from __future__ import annotations

from .protocol import DetectOutput, Detector


class YOLODetector:
    """YOLOv8 检测适配器骨架。"""

    def __init__(self, weights_path: str, confidence_threshold: float) -> None:
        self._weights_path = weights_path
        self._confidence_threshold = confidence_threshold
        # TODO(init): 接入 ultralytics 加载权重，记录 model_version。

    def detect(self, image_bytes: bytes) -> DetectOutput:
        # TODO(init): 解码 → 预处理 → 推理 → 坐标映射回原图坐标系。
        raise NotImplementedError("YOLO detector not implemented yet")
