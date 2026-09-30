"""图片解析与校验。

负责识别链路的前置图像处理：
  1. 格式校验：Pillow 识别格式必须落在允许白名单（jpeg/png/webp），
     防止伪装扩展名/非图像内容混入推理链路；
  2. 尺寸校验：超过配置上限按 INVALID_IMAGE 拒绝；
  3. EXIF 方向归一：输出方向校正后的宽高，后续 bbox 坐标以此为原图坐标系
     （03 文档第 7 节：bbox 为按 EXIF 方向统一后的原图像素坐标）。

异常约定：校验失败抛 ImageValidationError，由路由层映射 422 INVALID_IMAGE。
"""
from __future__ import annotations

import io
from dataclasses import dataclass

from PIL import Image, ImageOps, UnidentifiedImageError


class ImageValidationError(Exception):
    """图片校验失败：格式不支持 / 内容损坏 / 尺寸超限。"""

    def __init__(self, message: str) -> None:
        super().__init__(message)
        self.message = message


@dataclass
class ImageInfo:
    """校验通过后的图片信息（EXIF 方向已归一）。"""

    width: int
    height: int
    format_name: str
    exif_was_applied: bool


def inspect_image(image_bytes: bytes, allowed_formats: list[str]) -> ImageInfo:
    """校验并解析图片，返回方向归一后的宽高。

    校验失败抛 ImageValidationError；成功时以原始字节打开，不重编码。
    """
    try:
        im = Image.open(io.BytesIO(image_bytes))
        im.verify()
    except UnidentifiedImageError as e:
        raise ImageValidationError("unrecognized image format") from e
    except OSError as e:
        raise ImageValidationError(f"corrupted image: {e}") from e

    # verify 后需要重新打开才能读取属性。
    im = Image.open(io.BytesIO(image_bytes))
    fmt = (im.format or "").lower()
    if allowed_formats and fmt not in allowed_formats:
        raise ImageValidationError(
            f"unsupported image format: {fmt or 'unknown'} (allowed: {allowed_formats})"
        )

    width, height = im.size
    exif_was_applied = False
    exif = im.getexif()
    orientation = exif.get(0x0112) if exif is not None else None
    if orientation is not None and orientation in (5, 6, 7, 8):
        # 方向 5/6/7/8 会交换宽高，先归一再记录尺寸。
        transposed = ImageOps.exif_transpose(im)
        if transposed is not None:
            width, height = transposed.size
            exif_was_applied = True

    return ImageInfo(
        width=int(width),
        height=int(height),
        format_name=fmt,
        exif_was_applied=exif_was_applied,
    )
