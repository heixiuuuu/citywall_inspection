import io

from PIL import Image

from app.models.protocol import DetectOutput
from app.models.tiled import TiledDetector, positions
from app.schemas import BBox, Detection


def image_bytes(size):
    output = io.BytesIO()
    Image.new("RGB", size, "white").save(output, format="PNG")
    return output.getvalue()


def test_tiles_cover_edges_and_merge_overlapping_candidates():
    assert positions(260, 100) == [0, 75, 150, 160]

    class FakeDetector:
        def __init__(self):
            self.calls = 0

        def detect(self, data):
            image = Image.open(io.BytesIO(data))
            box = BBox(x1=70, y1=10, x2=90, y2=30) if self.calls == 0 else BBox(x1=10, y1=10, x2=30, y2=30)
            self.calls += 1
            item = Detection(detection_id="d0000", class_code="SURFACE_SPALLING", confidence=.8, bbox=box)
            return DetectOutput(*image.size, [item])

    detector = FakeDetector()
    output = TiledDetector(detector, 100).detect(image_bytes((160, 100)))
    assert detector.calls == 2
    assert (output.image_width, output.image_height) == (160, 100)
    assert len(output.detections) == 1
    assert output.detections[0].bbox == BBox(x1=70, y1=10, x2=90, y2=30)


def test_large_images_return_original_pixel_coordinates():
    class FakeDetector:
        def detect(self, data):
            image = Image.open(io.BytesIO(data))
            item = Detection(detection_id="d0000", class_code="SURFACE_SPALLING", confidence=.8,
                             bbox=BBox(x1=0, y1=0, x2=10, y2=10))
            return DetectOutput(*image.size, [item])

    output = TiledDetector(FakeDetector(), 100).detect(image_bytes((400, 200)))
    assert (output.image_width, output.image_height) == (400, 200)
    assert output.detections[0].bbox == BBox(x1=0, y1=0, x2=13, y2=13)
