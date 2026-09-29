"""识别编排服务。

调用方向：FastAPI route → recognition service → Detector adapter。
当前骨架按 03 文档约定“先用固定响应验证契约”，未接真实模型。
"""
from __future__ import annotations

import time

from .models.protocol import DetectOutput, Detector
from .schemas import RecognitionResponse


class RecognitionService:
    def __init__(self, detector: Detector, model_version: str) -> None:
        self._detector = detector
        self._model_version = model_version

    def recognize(self, attempt_id: str, image_bytes: bytes) -> RecognitionResponse:
        """对单张图片执行检测，返回原图坐标系中的候选。

        真实实现流程（03 文档）：
          1. 校验格式与尺寸（INVALID_IMAGE / IMAGE_TOO_LARGE）；
          2. 统一 EXIF 方向并记录送入识别的方向、尺寸；
          3. 调用 Detector 推理，把网络输入尺寸的框映射回传入图像尺寸；
          4. 失败时抛 RecognitionError（MODEL_UNAVAILABLE / INFERENCE_FAILED）。
        """
        started = time.monotonic()
        out: DetectOutput = self._detector.detect(image_bytes)
        latency_ms = int((time.monotonic() - started) * 1000)
        return RecognitionResponse(
            attempt_id=attempt_id,
            model_version=self._model_version,
            image_width=out.image_width,
            image_height=out.image_height,
            latency_ms=latency_ms,
            detections=out.detections,
        )
