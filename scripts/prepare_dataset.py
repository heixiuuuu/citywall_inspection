"""将 MSD 的 VOC 标注转换为 YOLO 数据集，按图片内容哈希划分训练、验证和测试集。"""
from __future__ import annotations

import argparse
import hashlib
import io
import json
import zipfile
from collections import Counter
from pathlib import Path
from xml.etree import ElementTree

from PIL import Image, ImageOps
import yaml

CODES = ["CRACK", "DISCOLOURATION", "EFFLORESCENCE", "SURFACE_DISSOLUTION", "MASONRY_LOSS", "FUNGUS", "MOSS", "PLANT", "CORROSION", "ARTISTIC"]
LABELS = {
    "cracks": "CRACK", "discolouration": "DISCOLOURATION",
    "efflorescence": "EFFLORESCENCE", "surface dissolution": "SURFACE_DISSOLUTION",
    "fabric loss of masonry units": "MASONRY_LOSS", "moss": "MOSS", "plant": "PLANT",
}


def annotation_image(image: Image.Image, expected_size: tuple[int, int]):
    if image.size == expected_size:
        return image.convert("RGB"), False
    normalized = ImageOps.exif_transpose(image)
    if normalized.size == expected_size:
        return normalized.convert("RGB"), True
    return None, False


def prepare(source: Path, output: Path):
    if (output / "data.yaml").exists():
        raise ValueError("output already contains a dataset; use a new output directory")
    try:
        import pillow_heif
        pillow_heif.register_heif_opener()
    except ImportError:
        pass
    counts, classes, skipped, normalizations = Counter(), Counter(), Counter(), Counter()
    records, seen = [], set()
    with zipfile.ZipFile(source / "yolo" / "MSD-dataset.zip") as archive:
        names = {Path(name).name.lower(): name for name in archive.namelist() if not name.startswith("__MACOSX")}
        annotations = sorted(name for name in archive.namelist() if name.lower().endswith(".xml") and not name.startswith("__MACOSX"))
        for annotation in annotations:
            document = ElementTree.fromstring(archive.read(annotation))
            filename = document.findtext("filename", "")
            image_path = names.get(Path(filename).name.lower())
            if not image_path:
                skipped["missing_image"] += 1
                continue
            raw = archive.read(image_path)
            try:
                with Image.open(io.BytesIO(raw)) as image:
                    expected_size = (int(document.findtext("size/width", "0")), int(document.findtext("size/height", "0")))
                    rgb, orientation_applied = annotation_image(image, expected_size)
                    if rgb is None:
                        skipped["dimension_mismatch"] += 1
                        continue
                    width, height = rgb.size
                    digest = hashlib.sha256(rgb.tobytes() + f"{width}:{height}".encode()).hexdigest()
                    if digest in seen:
                        skipped["duplicate_image"] += 1
                        continue
                    labels = []
                    for obj in document.findall("object"):
                        name = obj.findtext("name", "").strip().lower()
                        if name not in LABELS:
                            raise ValueError(f"unmapped label: {name}")
                        box = obj.find("bndbox")
                        x1, y1, x2, y2 = (float(box.findtext(key, "0")) for key in ("xmin", "ymin", "xmax", "ymax"))
                        original_box = (x1, y1, x2, y2)
                        x1, y1, x2, y2 = max(0, x1), max(0, y1), min(width, x2), min(height, y2)
                        if original_box != (x1, y1, x2, y2):
                            skipped["clipped_boxes"] += 1
                        if not (0 <= x1 < x2 <= width and 0 <= y1 < y2 <= height):
                            skipped["invalid_boxes"] += 1
                            continue
                        code = LABELS[name]
                        labels.append(f"{CODES.index(code)} {(x1+x2)/2/width:.8f} {(y1+y2)/2/height:.8f} {(x2-x1)/width:.8f} {(y2-y1)/height:.8f}")
                        classes[code] += 1
                    bucket = int(hashlib.sha256(("citywall:" + digest).encode()).hexdigest()[:8], 16) % 100
                    split = "train" if bucket < 80 else "val" if bucket < 90 else "test"
                    for kind in ("images", "labels"):
                        (output / kind / split).mkdir(parents=True, exist_ok=True)
                    rgb.save(output / "images" / split / f"{digest}.jpg", quality=95)
                    (output / "labels" / split / f"{digest}.txt").write_text("\n".join(labels), encoding="utf-8")
            except (OSError, ValueError) as error:
                if isinstance(error, ValueError):
                    raise
                skipped["unsupported_or_corrupt_image"] += 1
                continue
            seen.add(digest)
            if orientation_applied:
                normalizations["exif_orientation"] += 1
            counts[split] += 1
            records.append({"source": image_path, "annotation": annotation, "sha256": digest, "split": split, "orientation_applied": orientation_applied})
    if any(counts[split] == 0 for split in ("train", "val", "test")):
        raise ValueError("each dataset split must contain images")
    (output / "data.yaml").write_text(yaml.safe_dump({"path": str(output.resolve()), "train": "images/train", "val": "images/val", "test": "images/test", "names": dict(enumerate(CODES))}), encoding="utf-8")
    manifest = {"counts": dict(counts), "class_boxes": dict(classes), "skipped": dict(skipped), "normalizations": dict(normalizations), "images": records}
    (output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({key: value for key, value in manifest.items() if key != "images"}, ensure_ascii=False))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=Path(r"C:\a\school\毕设"))
    parser.add_argument("--output", type=Path, default=Path("data/yolo/dataset"))
    args = parser.parse_args()
    prepare(args.source, args.output)
