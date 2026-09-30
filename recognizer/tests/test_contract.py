"""契约测试：验证 /internal/v1/recognitions 与 03 文档及 Go 客户端一致。

运行：cd recognizer && python -m pytest tests/ -q
覆盖：
  - 成功响应关键字段（200 + RecognitionResponse）；
  - 参数缺失拒绝（422）；
  - 内部鉴权（require_token 开启时无/错 token → 401 + error.code）；
  - 图片超限（413 + IMAGE_TOO_LARGE，非 200，避免 Go 端误判）。
"""
from __future__ import annotations

import io

from fastapi.testclient import TestClient
from PIL import Image

from app.api.routes import settings as routes_settings
from app.main import app

client = TestClient(app)


def _png_bytes(width: int = 2, height: int = 2) -> bytes:
    buf = io.BytesIO()
    Image.new("RGB", (width, height), (200, 120, 80)).save(buf, "PNG")
    return buf.getvalue()


PNG_BYTES = _png_bytes()


def _post(**kwargs):
    data = {"attempt_id": "ra_101", "image_id": "img_1", "request_id": "req_1"}
    data.update(kwargs.pop("data", {}))
    return client.post(
        "/internal/v1/recognitions",
        files=kwargs.pop("files", {"image": ("sample.png", io.BytesIO(PNG_BYTES), "image/png")}),
        data=data,
        **kwargs,
    )


def test_healthz() -> None:
    resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.json()["status"] == "ok"


def test_recognition_success_contract() -> None:
    resp = _post()
    assert resp.status_code == 200
    body = resp.json()
    # 03 文档成功响应关键字段
    assert body["schema_version"] == "1"
    assert body["attempt_id"] == "ra_101"
    assert body["model_version"]
    assert body["image_width"] > 0 and body["image_height"] > 0
    assert isinstance(body["detections"], list)
    assert body["latency_ms"] >= 0


def test_recognition_missing_fields_rejected() -> None:
    resp = client.post(
        "/internal/v1/recognitions",
        files={"image": ("sample.png", io.BytesIO(PNG_BYTES), "image/png")},
        data={"attempt_id": "ra_1"},
    )
    # attempt_id/image_id 缺失属于参数错误
    assert resp.status_code == 422


def test_internal_auth_rejected(monkeypatch) -> None:
    monkeypatch.setattr(routes_settings.internal, "require_token", True)
    monkeypatch.setattr(routes_settings.internal, "token", "secret-token")
    try:
        # 无 token → 401，且错误体可被 Go 客户端解析。
        resp = _post()
        assert resp.status_code == 401
        assert resp.json()["error"]["code"] == "UNAUTHORIZED"
        # 错误 token → 401。
        resp2 = _post(headers={"X-Internal-Token": "wrong"})
        assert resp2.status_code == 401
        # 正确 token → 200。
        resp3 = _post(headers={"X-Internal-Token": "secret-token"})
        assert resp3.status_code == 200
    finally:
        monkeypatch.setattr(routes_settings.internal, "require_token", False)


def test_image_too_large(monkeypatch) -> None:
    monkeypatch.setattr(routes_settings.recognition, "max_image_bytes", 10)
    resp = _post()
    assert resp.status_code == 413
    assert resp.json()["error"]["code"] == "IMAGE_TOO_LARGE"


def test_empty_image_invalid() -> None:
    resp = client.post(
        "/internal/v1/recognitions",
        files={"image": ("empty.png", io.BytesIO(b""), "image/png")},
        data={"attempt_id": "ra_1", "image_id": "img_1", "request_id": "req_1"},
    )
    assert resp.status_code == 422
    assert resp.json()["error"]["code"] == "INVALID_IMAGE"
