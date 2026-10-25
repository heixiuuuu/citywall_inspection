"""识别服务输入输出契约（与 03 文档第 7 节一致）。

- bbox 为按 EXIF 方向统一后的原图像素坐标，左上角原点，x1 < x2 <= width, y1 < y2 <= height；
- detections: [] 表示模型未检出，不能替代人员填写“正常”；
- confidence 仅代表检测器候选分数，不是病害严重度。
"""
from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field

SCHEMA_VERSION = "1"


class BBox(BaseModel):
    x1: int
    y1: int
    x2: int
    y2: int


class Detection(BaseModel):
    detection_id: str
    class_code: str
    confidence: float = Field(ge=0.0, le=1.0)
    bbox: BBox


class VisionAnalysis(BaseModel):
    status: Literal["SUCCEEDED", "DISABLED", "FAILED"]
    model: str = ""
    description: str = Field(default="", max_length=3000)
    possible_causes: list[str] = Field(default_factory=list, max_length=10)
    recommendations: list[str] = Field(default_factory=list, max_length=10)
    error_code: str = ""


class RecognitionResponse(BaseModel):
    schema_version: str = SCHEMA_VERSION
    attempt_id: str
    model_version: str
    image_width: int
    image_height: int
    latency_ms: int
    detections: list[Detection] = []
    analysis: VisionAnalysis | None = None


ErrorCode = Literal[
    "INVALID_IMAGE",
    "IMAGE_TOO_LARGE",
    "MODEL_UNAVAILABLE",
    "INFERENCE_FAILED",
]


class RecognitionError(BaseModel):
    error: dict[str, str] = Field(
        description="含 code 与 message；code 必须属于 03 文档约定集合"
    )
    request_id: str
