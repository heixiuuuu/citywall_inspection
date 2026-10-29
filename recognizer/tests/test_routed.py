from concurrent.futures import ThreadPoolExecutor
import time

import pytest

from app.models.protocol import DetectOutput
from app.models.routed import ClassRoutedDetector
from app.models.yolo import DetectorError
from app.schemas import BBox, Detection


def detection(code, score):
    return Detection(detection_id="d0000", class_code=code, confidence=score, bbox=BBox(x1=0, y1=0, x2=10, y2=10))


class Detector:
    def __init__(self, items, size=(20, 20)):
        self.output = DetectOutput(*size, items)

    def detect(self, image_bytes):
        return self.output


def test_category_routing_replaces_only_selected_classes_and_preserves_inputs():
    primary = Detector([detection("CRACK", .9), detection("PLANT", .8)])
    specialist = Detector([detection("CRACK", .7), detection("PLANT", .6)])
    routed = ClassRoutedDetector(primary, specialist, frozenset({"CRACK"}))
    output = routed.detect(b"image")
    assert [(item.class_code, item.confidence) for item in output.detections] == [("PLANT", .8), ("CRACK", .7)]
    assert len({item.detection_id for item in output.detections}) == 2
    assert primary.output.detections[1].detection_id == "d0000"
    assert (output.image_width, output.image_height) == (20, 20)


def test_coordinate_mismatch_does_not_return_mixed_evidence():
    routed = ClassRoutedDetector(Detector([]), Detector([], (30, 20)), frozenset({"CRACK"}))
    with pytest.raises(DetectorError, match="coordinate systems"):
        routed.detect(b"image")


def test_spalling_detector_preserves_existing_routed_categories():
    primary = Detector([detection("PLANT", .8), detection("CRACK", .4)])
    specialist = Detector([detection("CRACK", .7)])
    spalling = Detector([detection("SURFACE_SPALLING", .6)])
    routed = ClassRoutedDetector(ClassRoutedDetector(primary, specialist, frozenset({"CRACK"})), spalling, frozenset({"SURFACE_SPALLING"}))
    result = routed.detect(b"image")
    assert [(item.class_code, item.confidence) for item in result.detections] == [("PLANT", .8), ("CRACK", .7), ("SURFACE_SPALLING", .6)]
    assert len({item.detection_id for item in result.detections}) == 3


def test_specialist_failure_is_not_silently_replaced_with_primary_output():
    class FailedDetector:
        def detect(self, image_bytes):
            raise DetectorError("MODEL_UNAVAILABLE", "missing specialist")
    routed = ClassRoutedDetector(Detector([detection("CRACK", .9)]), FailedDetector(), frozenset({"CRACK"}))
    with pytest.raises(DetectorError) as error:
        routed.detect(b"image")
    assert error.value.code == "MODEL_UNAVAILABLE"


@pytest.mark.parametrize("codes", [frozenset(), frozenset({"W_E"})])
def test_invalid_routing_categories_are_rejected(codes):
    with pytest.raises(ValueError):
        ClassRoutedDetector(Detector([]), Detector([]), codes)


def test_concurrent_images_do_not_interleave_model_access():
    calls = []
    class RecordingDetector:
        def __init__(self, name):
            self.name = name
        def detect(self, image_bytes):
            calls.append((self.name, image_bytes))
            time.sleep(.001)
            return DetectOutput(20, 20, [])
    routed = ClassRoutedDetector(RecordingDetector("primary"), RecordingDetector("specialist"), frozenset({"CRACK"}))
    with ThreadPoolExecutor(max_workers=4) as pool:
        list(pool.map(routed.detect, [bytes([index]) for index in range(12)]))
    for index in range(0, len(calls), 2):
        assert calls[index][0] == "primary" and calls[index+1][0] == "specialist"
        assert calls[index][1] == calls[index+1][1]
