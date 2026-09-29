"""契约测试：验证 /internal/v1/recognitions 的固定响应与 03 文档一致。

运行：cd recognizer && python -m pytest tests/ -q
"""
from __future__ import annotations

import io

from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)

PNG_BYTES = (
    b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
    b"\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89"
    b"\x00\x00\x00\rIDAT\x08\xd7c\xf8\xff\xff?\x00\x05\xfe\x02\xfe\xdc\xcc"
    b"\x59\xe7\x00\x00\x00\x00IEND\xaeB`\x82"
)


def test_healthz() -> None:
    resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.json()["status"] == "ok"


def test_recognition_success_contract() -> None:
    resp = client.post(
        "/internal/v1/recognitions",
        files={"image": ("sample.png", io.BytesIO(PNG_BYTES), "image/png")},
        data={"attempt_id": "ra_101", "image_id": "img_1", "request_id": "req_1"},
    )
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
