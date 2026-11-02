import sys
from pathlib import Path
from types import SimpleNamespace

from PIL import Image
import pytest
import yaml

from scripts.evaluate_operating_point import evaluate, read_truth, score_image
from app.config import RecognitionConfig
from app.models.factory import build_detector
from app.models.yolo import YOLODetector
from app.service import RecognitionService


@pytest.fixture
def deployment_case(tmp_path, monkeypatch):
    calls = []

    class Model:
        def __init__(self, role):
            self.role = role
            self.names = {0: "CRACK", 1: "PLANT", 2: "FUNGUS", 3: "SURFACE_SPALLING"}
            self.predictor = None

        def predict(self, image, conf, verbose=False, **kwargs):
            calls.append((self.role, image.size, conf, kwargs))
            self.predictor = SimpleNamespace(device="cpu", args=SimpleNamespace(
                **({"imgsz": 960 if self.role == "specialist" else 640, "iou": .7, "device": "cpu"} | kwargs)))
            rows = {
                "primary": [([80, 80, 90, 90], 0, .9), ([30.1, 30.1, 50.2, 50.2], 1, .87654), ([15, 15, 20, 20], 2, .7)],
                "specialist": [([.2, .2, 10.6, 10.6], 0, .91234), ([5, 5, 6, 6], 1, .9)],
                "spalling": [([60, 40, 70, 50], 3, .8)],
            }[self.role]
            boxes = SimpleNamespace(xyxy=[row[0] for row in rows], cls=[row[1] for row in rows], conf=[row[2] for row in rows])
            return [SimpleNamespace(boxes=boxes, names=self.names)]

    def load(detector):
        if detector._model is None:
            detector._model = Model(Path(detector.weights_path).stem)
        return detector._model

    monkeypatch.setattr(YOLODetector, "_load_model", load)
    monkeypatch.setitem(sys.modules, "torch", SimpleNamespace(__version__="fixture", version=SimpleNamespace(cuda=None),
                        get_num_threads=lambda: 1, cuda=SimpleNamespace(device_count=lambda: 0)))
    monkeypatch.setitem(sys.modules, "ultralytics", SimpleNamespace(__version__="fixture"))
    for role in ("primary", "specialist", "spalling"):
        (tmp_path / f"{role}.pt").write_bytes(role.encode())
    for split in ("val", "test"):
        (tmp_path / "images" / split).mkdir(parents=True)
        (tmp_path / "labels" / split).mkdir(parents=True)
        exif = Image.Exif()
        exif[274] = 6
        Image.new("RGB", (200, 100), "white").save(tmp_path / "images" / split / "sample.jpg", exif=exif)
        (tmp_path / "labels" / split / "sample.txt").write_text("0 .055 .0275 .11 .055\n1 .4 .2 .2 .1\n")
    data = tmp_path / "data.yaml"
    data.write_text(yaml.safe_dump({"path": str(tmp_path), "val": "images/val", "test": "images/test", "names": ["CRACK", "PLANT"]}))
    config = RecognitionConfig(weights_path=str(tmp_path / "primary.pt"), specialist_weights_path=str(tmp_path / "specialist.pt"),
                               specialist_class_codes=["CRACK"], spalling_weights_path=str(tmp_path / "spalling.pt"),
                               spalling_confidence_threshold=.25, spalling_tile_size=128)
    return data, config, calls


@pytest.mark.parametrize("split", ["val", "test"])
def test_evaluation_matches_service_routing_tiles_orientation_and_rounding(deployment_case, split):
    data, config, calls = deployment_case
    result = evaluate(data, split, config, ["CRACK", "PLANT"])
    case = result["per_image"][0]
    service = RecognitionService(build_detector(config), config.model_version)
    expected = service.recognize("comparison", (data.parent / "images" / split / "sample.jpg").read_bytes())
    assert case["predictions"] == [item.model_dump() for item in expected.detections]
    assert (case["width"], case["height"], case["source_exif_orientation"]) == (100, 200, 6)
    assert result["aggregate"]["tp"] == 2
    assert result["aggregate"]["fp"] == result["aggregate"]["fn"] == 0
    assert case["unassessed_predictions"] == {"FUNGUS": 1, "SURFACE_SPALLING": 2}
    assert result["candidate_burden"]["total"] == 5
    crack = next(item for item in case["predictions"] if item["class_code"] == "CRACK")
    assert crack["bbox"] == {"x1": 0, "y1": 0, "x2": 11, "y2": 11}
    assert crack["confidence"] == .9123
    assert {row["prediction_index"] for row in case["matches"]} == {0, 2}
    assert len(result["weights"]) == 3 and len(result["timing"]["warmup_recognition_ms"]) == 1
    assert result["runtime_members"][1]["parameters"]["imgsz"] == 960
    assert calls[0][:2] == ("primary", (100, 200))
    assert all(conf == .25 for role, size, conf, kwargs in calls if role == "spalling")


def test_single_weight_control_and_inference_overrides(deployment_case):
    from dataclasses import replace
    data, config, calls = deployment_case
    config = replace(config, specialist_weights_path="", specialist_class_codes=[], spalling_weights_path="",
                     device="cpu", imgsz=320, nms_iou=.6)
    result = evaluate(data, "val", config, ["CRACK", "PLANT"], warmup=0)
    assert len(result["weights"]) == 1
    assert result["aggregate"]["tp"] == 1 and result["aggregate"]["fp"] == result["aggregate"]["fn"] == 1
    assert calls == [("primary", (100, 200), .35, {"device": "cpu", "imgsz": 320, "iou": .6})]


def test_annotation_coverage_is_explicit_and_cannot_hide_ground_truth():
    with pytest.raises(ValueError, match="outside declared"):
        read_truth(b"0 .5 .5 .2 .2", {0: "CRACK"}, 100, 100, {"PLANT"})
    with pytest.raises(ValueError, match="invalid label"):
        read_truth(b"0 nan .5 .2 .2", {0: "CRACK"}, 100, 100, {"CRACK"})


def test_annotated_absent_class_predictions_are_false_positives():
    from app.schemas import BBox, Detection
    item = Detection(detection_id="d0", class_code="CRACK", confidence=.8, bbox=BBox(x1=0, y1=0, x2=10, y2=10))
    scored = score_image([], [item], {"CRACK"}, .5)
    assert scored["counts"] == {"CRACK": {"tp": 0, "fp": 1, "fn": 0}}
    assert scored["unassessed_predictions"] == {}


def test_missing_labels_fail_instead_of_becoming_negative_images(deployment_case):
    data, config, calls = deployment_case
    (data.parent / "labels/val/sample.txt").unlink()
    with pytest.raises(FileNotFoundError):
        evaluate(data, "val", config, ["CRACK", "PLANT"], warmup=0)


def test_input_rejection_is_preflighted_and_reported_without_false_negatives(deployment_case):
    from dataclasses import replace
    data, config, calls = deployment_case
    content = (data.parent / "images/val/sample.jpg").read_bytes()
    (data.parent / "images/val/zz-oversized.jpg").write_bytes(content + b"\0" * 100)
    (data.parent / "labels/val/zz-oversized.txt").write_text("0 .055 .0275 .11 .055\n")
    config = replace(config, max_image_bytes=len(content) + 50)
    with pytest.raises(ValueError, match="service size limit"):
        evaluate(data, "val", config, ["CRACK", "PLANT"], warmup=0)
    assert calls == []
    result = evaluate(data, "val", config, ["CRACK", "PLANT"], warmup=0, allow_rejected_inputs=True)
    assert result["images"] == 1 and result["aggregate"]["tp"] == 2 and result["aggregate"]["fn"] == 0
    admission = result["input_admission"]
    assert admission["coverage"] == .5 and admission["requested_images"] == 2
    assert admission["rejected"][0]["reason"] == "IMAGE_TOO_LARGE"
    assert admission["rejected"][0]["unassessed_label_boxes"] == 1
    assert result["dataset_sha256"] != result["input_dataset_sha256"]
