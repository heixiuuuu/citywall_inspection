"""图像校验契约测试：格式白名单、EXIF 方向归一、真实尺寸。

覆盖 03 文档第 7 节“bbox 为按 EXIF 方向统一后的原图像素坐标”，
以及 todo「图像校验完整化」的验收口径：非法格式 422、EXIF 方向归一。
"""
from __future__ import annotations

import io

from PIL import Image, ImageOps

from fastapi.testclient import TestClient

from app.main import app
from app.api.routes import settings as routes_settings

client = TestClient(app)


def _png_bytes(width: int = 2, height: int = 2) -> bytes:
    buf = io.BytesIO()
    Image.new("RGB", (width, height), (200, 120, 80)).save(buf, "PNG")
    return buf.getvalue()


def _jpeg_with_orientation(width: int, height: int, orientation: int) -> bytes:
    buf = io.BytesIO()
    im = Image.new("RGB", (width, height), (180, 160, 140))
    exif = Image.Exif()
    exif[0x0112] = orientation
    im.save(buf, "JPEG", exif=exif)
    return buf.getvalue()


def _post(files: dict) -> "object":
    return client.post(
        "/internal/v1/recognitions",
        files=files,
        data={"attempt_id": "ra_img", "image_id": "img_1", "request_id": "req_img"},
    )


def test_png_accepted_with_real_dimensions() -> None:
    resp = _post({"image": ("wall.png", io.BytesIO(_png_bytes(8, 5)), "image/png")})
    assert resp.status_code == 200
    body = resp.json()
    # 尺寸来自真实图片解析（EXIF 归一后），不再是指定占位值。
    assert body["image_width"] == 8
    assert body["image_height"] == 5


def test_unsupported_format_rejected() -> None:
    # 伪装成 png 的纯文本字节 → Pillow 无法解析 → 422 INVALID_IMAGE。
    bogus = b"this is definitely not an image at all........"
    resp = _post({"image": ("bogus.png", io.BytesIO(bogus), "image/png")})
    assert resp.status_code == 422
    assert resp.json()["error"]["code"] == "INVALID_IMAGE"


def test_gif_format_rejected_by_whitelist(monkeypatch) -> None:
    buf = io.BytesIO()
    Image.new("RGB", (4, 4)).save(buf, "GIF")
    # 白名单当前为 jpeg/png/webp，GIF 不在其中。
    resp = _post({"image": ("anim.gif", io.BytesIO(buf.getvalue()), "image/gif")})
    assert resp.status_code == 422
    assert resp.json()["error"]["code"] == "INVALID_IMAGE"


def test_exif_orientation_applied() -> None:
    # orientation=6 表示顺时针 90°，宽高交换；5/6/7/8 均交换。
    data = _jpeg_with_orientation(3, 7, 6)
    resp = _post({"image": ("oriented.jpg", io.BytesIO(data), "image/jpeg")})
    assert resp.status_code == 200
    body = resp.json()
    assert body["image_width"] == 7
    assert body["image_height"] == 3


def test_exif_orientation_1_keeps_dimensions() -> None:
    data = _jpeg_with_orientation(3, 7, 1)
    resp = _post({"image": ("normal.jpg", io.BytesIO(data), "image/jpeg")})
    assert resp.status_code == 200
    body = resp.json()
    assert body["image_width"] == 3
    assert body["image_height"] == 7


def test_png_dimensions_contract_still_valid() -> None:
    # 原有 1x1 PNG 成功路径不回归。
    resp = _post({"image": ("sample.png", io.BytesIO(_png_bytes(1, 1)), "image/png")})
    assert resp.status_code == 200
    body = resp.json()
    assert body["image_width"] == 1 and body["image_height"] == 1
