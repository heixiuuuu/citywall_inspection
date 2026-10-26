"""YOLO 真实推理适配器测试：坐标映射、类别映射、阈值过滤、零检测、缺权重。

不依赖真实权重：注入假模型（duck typing）验证坐标系契约与错误路径。
"""
from __future__ import annotations

import io

import pytest
from PIL import Image

from app.models.yolo import DetectorError, YOLODetector, resolve_weights_path


def _png_bytes(size=(640, 480), color=(120, 120, 120)) -> bytes:
    buf = io.BytesIO()
    Image.new("RGB", size, color).save(buf, format="PNG")
    return buf.getvalue()


class _FakeBoxes:
    def __init__(self, items):
        self.xyxy = [it[0] for it in items]
        self.cls = [it[1] for it in items]
        self.conf = [it[2] for it in items]
        self._items = items

    def __len__(self):
        return len(self._items)

    def __iter__(self):
        return iter(self._items)


class _FakeResult:
    def __init__(self, boxes):
        self.boxes = boxes
        self.names = {0: "CRACK", 3: "SURFACE_DISSOLUTION"}


class _FakeModel:
    def __init__(self, results):
        self._results = results
        self.loaded_at = None

    def predict(self, image, conf=None, verbose=False):
        self.loaded_at = image.size
        return self._results


def test_detect_maps_boxes_to_original_pixel_coords():
    fake = _FakeModel(
        [_FakeResult(_FakeBoxes([([10.2, 20.7, 300.8, 410.4], 0, 0.91), ([5.0, 6.0, 60.0, 66.0], 3, 0.62)]))]
    )
    det = YOLODetector("unused.pt", confidence_threshold=0.5, class_codes=frozenset({"CRACK", "SURFACE_DISSOLUTION"}))
    det._model = fake  # 注入假模型，跳过加载

    out = det.detect(_png_bytes(size=(640, 480)))
    assert out.image_width == 640
    assert out.image_height == 480
    assert len(out.detections) == 2

    d0, d1 = out.detections
    assert d0.class_code == "CRACK"
    assert d0.bbox.x1 == 10 and d0.bbox.y1 == 21
    assert d0.bbox.x2 == 301 and d0.bbox.y2 == 410
    assert d0.confidence == pytest.approx(0.91, abs=1e-3)
    assert d1.class_code == "SURFACE_DISSOLUTION"
    assert d1.bbox.x1 == 5 and d1.bbox.y1 == 6


def test_detect_filters_below_confidence_threshold():
    fake = _FakeModel([_FakeResult(_FakeBoxes([([0, 0, 50, 50], 0, 0.88), ([0, 0, 50, 50], 0, 0.30)]))])
    det = YOLODetector("unused.pt", confidence_threshold=0.5)
    det._model = fake
    out = det.detect(_png_bytes())
    assert len(out.detections) == 1
    assert out.detections[0].confidence == pytest.approx(0.88)


def test_detect_zero_detections_returns_empty_not_normal():
    fake = _FakeModel([_FakeResult(_FakeBoxes([]))])
    det = YOLODetector("unused.pt", confidence_threshold=0.5)
    det._model = fake
    out = det.detect(_png_bytes())
    assert out.detections == []
    assert out.image_width == 640 and out.image_height == 480


def test_detect_skips_unknown_class_codes():
    # 未知类别没有名称，不得根据数字编号猜测业务类别。
    fake = _FakeModel([_FakeResult(_FakeBoxes([([0, 0, 50, 50], 99, 0.9)]))])
    det = YOLODetector("unused.pt", confidence_threshold=0.1, class_codes=frozenset({"CRACK"}))
    det._model = fake
    out = det.detect(_png_bytes())
    assert out.detections == []


def test_detect_missing_weights_raises_model_unavailable(tmp_path):
    det = YOLODetector(str(tmp_path / "nope.pt"))
    with pytest.raises(DetectorError) as exc:
        det.detect(_png_bytes())
    assert exc.value.code == "MODEL_UNAVAILABLE"


def test_model_class_order_does_not_change_business_meaning():
    result = _FakeResult(_FakeBoxes([([0, 0, 40, 40], 0, 0.9), ([50, 50, 90, 90], 1, 0.8)]))
    result.names = {0: "MOSS", 1: "CRACK"}
    detector = YOLODetector("unused.pt")
    detector._model = _FakeModel([result])
    assert [item.class_code for item in detector.detect(_png_bytes()).detections] == ["MOSS", "CRACK"]


def test_unknown_model_name_is_not_assigned_by_numeric_index():
    result = _FakeResult(_FakeBoxes([([0, 0, 40, 40], 0, 0.9)]))
    result.names = {0: "person"}
    detector = YOLODetector("unused.pt")
    detector._model = _FakeModel([result])
    assert detector.detect(_png_bytes()).detections == []


def test_exif_orientation_applied_before_inference():
    # 构造带 EXIF 方向 6（旋转 90°）的 JPEG，验证推理坐标系已归一（宽高交换）。
    buf = io.BytesIO()
    im = Image.new("RGB", (200, 100), (10, 10, 10))
    exif = im.getexif()
    exif[0x0112] = 6
    im.save(buf, format="JPEG", exif=exif.tobytes())

    fake = _FakeModel([_FakeResult(_FakeBoxes([([10, 10, 90, 90], 0, 0.9)]))])
    det = YOLODetector("unused.pt", confidence_threshold=0.5)
    det._model = fake
    out = det.detect(buf.getvalue())
    # 方向 6：原始 200x100 → 归一后 100x200
    assert out.image_width == 100
    assert out.image_height == 200
    assert fake.loaded_at == (100, 200)


def test_resolve_weights_path_absolute_and_relative(tmp_path):
    assert resolve_weights_path(str(tmp_path / "best.pt")).is_absolute()
    p = resolve_weights_path("models/whatever.pt")
    assert p.name == "whatever.pt"


@pytest.mark.parametrize("names, accepted", [({0: "MOSS", 1: "CRACK"}, True), ({0: "person"}, False)])
def test_model_load_validates_semantic_names(tmp_path, monkeypatch, names, accepted):
    import sys
    from types import SimpleNamespace
    weights = tmp_path / "model.pt"
    weights.write_bytes(b"fixture")
    model = SimpleNamespace(names=names)
    monkeypatch.setitem(sys.modules, "ultralytics", SimpleNamespace(YOLO=lambda _: model))
    detector = YOLODetector(str(weights))
    if accepted:
        assert detector._load_model() is model
    else:
        with pytest.raises(DetectorError) as error:
            detector._load_model()
        assert error.value.code == "MODEL_UNAVAILABLE"
