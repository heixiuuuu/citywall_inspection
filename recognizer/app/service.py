"""识别编排服务。

调用方向：FastAPI route → recognition service → Detector adapter。
"""
from __future__ import annotations

import time

from .images import inspect_image
from .models.protocol import DetectOutput, Detector
from .schemas import RecognitionResponse


class RecognitionService:
    def __init__(self, detector: Detector, model_version: str, analyzer=None) -> None:
        self._detector = detector
        self._model_version = model_version
        self._analyzer = analyzer

    def recognize(
        self,
        attempt_id: str,
        image_bytes: bytes,
        allowed_formats: list[str] | None = None,
    ) -> RecognitionResponse:
        """对单张图片执行检测，返回原图坐标系中的候选。

        真实实现流程（03 文档）：
          1. 校验格式与尺寸（INVALID_IMAGE / IMAGE_TOO_LARGE）；
          2. 统一 EXIF 方向并记录送入识别的方向、尺寸；
          3. 调用 Detector 推理，把网络输入尺寸的框映射回传入图像尺寸；
          4. 失败时抛 RecognitionError（MODEL_UNAVAILABLE / INFERENCE_FAILED）。
        """
        started = time.monotonic()
        # 前置图像校验：格式白名单 + 内容可解析 + EXIF 方向归一。
        inspect_image(image_bytes, allowed_formats or [])
        out: DetectOutput = self._detector.detect(image_bytes)
        analysis = self._analyzer.analyze(image_bytes, [d.class_code for d in out.detections]) if self._analyzer else None
        latency_ms = int((time.monotonic() - started) * 1000)
        return RecognitionResponse(
            attempt_id=attempt_id,
            model_version=self._model_version,
            image_width=out.image_width,
            image_height=out.image_height,
            latency_ms=latency_ms,
            detections=out.detections,
            analysis=analysis,
        )
