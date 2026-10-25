"""识别服务配置。

配置来源（高 → 低）：
  1. 环境变量 RECOGNIZER_CONFIG 指定的文件
  2. ./conf/recognizer.json（仓库根下）
  3. ../conf/recognizer.json（从 recognizer/ 目录启动时）

"""
from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from pathlib import Path

_DEFAULT_CANDIDATES = (
    "conf/recognizer.json",
    "../conf/recognizer.json",
)


@dataclass
class RecognitionConfig:
    model_version: str = "yolo_baseline_001"
    detector_mode: str = "yolo"
    weights_path: str = "data/yolo/weights/best.pt"
    confidence_threshold: float = 0.35
    max_image_bytes: int = 10 * 1024 * 1024
    allowed_formats: list[str] = field(default_factory=lambda: ["jpeg", "png", "webp"])


@dataclass
class InternalConfig:
    require_token: bool = False
    token: str = ""


@dataclass
class VisionConfig:
    base_url: str = ""
    model: str = ""
    api_key: str = ""
    timeout_s: int = 45


@dataclass
class AppConfig:
    name: str = "city-wall-recognizer"
    env: str = "dev"


@dataclass
class Settings:
    vision: VisionConfig = field(default_factory=VisionConfig)
    app: AppConfig = field(default_factory=AppConfig)
    server: dict = field(default_factory=lambda: {"host": "0.0.0.0", "port": 8000})
    recognition: RecognitionConfig = field(default_factory=RecognitionConfig)
    internal: InternalConfig = field(default_factory=InternalConfig)


def _find_config_file() -> Path:
    override = os.getenv("RECOGNIZER_CONFIG")
    candidates = [override] if override else list(_DEFAULT_CANDIDATES)
    for cand in candidates:
        if not cand:
            continue
        path = Path(cand)
        if path.is_file():
            return path
    raise FileNotFoundError(
        "recognizer config not found; set RECOGNIZER_CONFIG or place conf/recognizer.json"
    )


def load_settings() -> Settings:
    path = _find_config_file()
    raw = json.loads(path.read_text(encoding="utf-8"))
    s = Settings()
    if "app" in raw:
        s.app = AppConfig(**raw["app"])
    if "server" in raw:
        s.server = raw["server"]
    if "recognition" in raw:
        s.recognition = RecognitionConfig(**raw["recognition"])
    if "internal" in raw:
        s.internal = InternalConfig(**raw["internal"])
    if "vision" in raw:
        s.vision = VisionConfig(**raw["vision"])
    s.vision.base_url = os.getenv("VLM_BASE_URL", s.vision.base_url)
    s.vision.model = os.getenv("VLM_MODEL", s.vision.model)
    s.vision.api_key = os.getenv("VLM_API_KEY", s.vision.api_key)
    s.recognition.weights_path = os.getenv("RECOGNIZER_WEIGHTS", s.recognition.weights_path)
    s.recognition.detector_mode = os.getenv("RECOGNIZER_MODE", s.recognition.detector_mode)
    token = os.getenv("RECOGNIZER_TOKEN")
    if token:
        s.internal.token = token
        s.internal.require_token = True
    if s.recognition.detector_mode not in ("dev", "yolo"):
        raise ValueError("detector_mode must be dev or yolo")
    return s


settings = load_settings()
