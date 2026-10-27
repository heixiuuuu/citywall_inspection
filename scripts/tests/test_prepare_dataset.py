import importlib.util
from pathlib import Path

import pytest
from PIL import Image

spec = importlib.util.spec_from_file_location("prepare_dataset", Path(__file__).resolve().parents[1] / "prepare_dataset.py")
prepare_dataset = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare_dataset)


@pytest.mark.parametrize("orientation, red_corner", [(6, (1, 0)), (8, (0, 3))])
def test_annotation_dimensions_follow_exif_coordinates(orientation, red_corner):
    image = Image.new("RGB", (4, 2), "blue")
    image.putpixel((0, 0), (255, 0, 0))
    image.getexif()[274] = orientation
    normalized, applied = prepare_dataset.annotation_image(image, (2, 4))
    assert applied and normalized.size == (2, 4)
    assert normalized.getpixel(red_corner) == (255, 0, 0)


def test_matching_annotation_coordinates_keep_original_pixels():
    image = Image.new("RGB", (4, 2), "blue")
    image.putpixel((0, 0), (255, 0, 0))
    image.getexif()[274] = 8
    normalized, applied = prepare_dataset.annotation_image(image, image.size)
    assert not applied and normalized.size == image.size
    assert normalized.tobytes() == image.tobytes()


def test_unexplained_dimension_mismatch_is_not_resized():
    normalized, applied = prepare_dataset.annotation_image(Image.new("RGB", (4, 2)), (5, 3))
    assert normalized is None and not applied
