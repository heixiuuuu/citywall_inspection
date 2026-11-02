"""核查 MSD 转换数据的类别、标注一致性及跨集合近似图像。"""
import argparse
from collections import Counter, defaultdict
import hashlib
import itertools
import json
from pathlib import Path
import sys
from xml.etree import ElementTree
import zipfile

from PIL import Image, ImageStat
import yaml

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from scripts.evaluate_operating_point import file_hash, overlap, read_truth
from scripts.prepare_dataset import LABELS


def difference_hash(image):
    pixels = image.convert("L").resize((9, 8)).tobytes()
    return sum((pixels[y * 9 + x] > pixels[y * 9 + x + 1]) << (y * 8 + x) for y in range(8) for x in range(8))


def audit(root, source_archive=None, max_image_bytes=10 * 1024 * 1024):
    root = root.resolve()
    manifest = json.loads((root / "manifest.json").read_text(encoding="utf-8"))
    data = yaml.safe_load((root / "data.yaml").read_text(encoding="utf-8"))
    names = {int(key): value for key, value in data["names"].items()}
    coverage = set(LABELS.values())
    counts, class_counts, negatives, fingerprint_groups = Counter(), defaultdict(Counter), Counter(), defaultdict(list)
    files, issues, duplicate_boxes, oversized = [], [], [], []
    archive = zipfile.ZipFile(source_archive) if source_archive else None
    try:
        for record in manifest["images"]:
            split, identity = record["split"], record["sha256"]
            image_path, label_path = root / "images" / split / f"{identity}.jpg", root / "labels" / split / f"{identity}.txt"
            content = image_path.read_bytes()
            labels = label_path.read_bytes()
            with Image.open(image_path) as image:
                width, height = image.size
                image.load()
                orientation = image.getexif().get(274, 1)
                dhash = difference_hash(image)
                deviation = ImageStat.Stat(image.convert("L").resize((64, 64))).stddev[0]
            truth = read_truth(labels, names, width, height, coverage)
            if len(content) > max_image_bytes:
                oversized.append({"image": f"{split}/{identity}.jpg", "byte_size": len(content), "unassessed_label_boxes": len(truth)})
            counts[split] += 1
            negatives[split] += not truth
            class_counts[split].update(code for code, box in truth)
            key = f"{split}/{identity}.jpg"
            fingerprint_groups[hashlib.sha256(content).hexdigest()].append(key)
            if orientation != 1:
                issues.append({"image": key, "issue": "converted_image_has_orientation", "orientation": orientation})
            for left, right in itertools.combinations(range(len(truth)), 2):
                if truth[left][0] == truth[right][0] and overlap(truth[left][1], truth[right][1]) >= .98:
                    duplicate_boxes.append({"image": key, "label_indices": [left, right], "class_code": truth[left][0]})
            if archive:
                document = ElementTree.fromstring(archive.read(record["annotation"]))
                expected_size = (int(document.findtext("size/width")), int(document.findtext("size/height")))
                expected = []
                if expected_size != (width, height):
                    issues.append({"image": key, "issue": "annotation_dimension_mismatch", "annotation_size": expected_size})
                for obj in document.findall("object"):
                    code = LABELS[obj.findtext("name", "").strip().lower()]
                    box = obj.find("bndbox")
                    x1, y1, x2, y2 = (float(box.findtext(name)) for name in ("xmin", "ymin", "xmax", "ymax"))
                    x1, y1, x2, y2 = max(0, x1), max(0, y1), min(width, x2), min(height, y2)
                    if x1 < x2 and y1 < y2:
                        expected.append((code, [x1, y1, x2, y2]))
                if len(truth) != len(expected) or any(a[0] != b[0] or max(abs(x-y) for x, y in zip(a[1], b[1])) > max(width, height) * 2e-8
                                                       for a, b in zip(truth, expected)):
                    issues.append({"image": key, "issue": "converted_labels_differ_from_source_voc"})
            files.append({"image": key, "source": record["source"], "split": split, "dhash": dhash,
                          "grayscale_stddev": deviation, "image_sha256": hashlib.sha256(content).hexdigest(),
                          "label_sha256": hashlib.sha256(labels).hexdigest()})
    finally:
        if archive:
            archive.close()
    near_pairs = []
    for left, right in itertools.combinations(files, 2):
        if left["split"] == right["split"]:
            continue
        distance = (left["dhash"] ^ right["dhash"]).bit_count()
        if distance <= 4:
            near_pairs.append({"left": left["image"], "right": right["image"], "left_source": left["source"],
                               "right_source": right["source"], "dhash_distance": distance})
    exact_groups = [group for group in fingerprint_groups.values() if len({item.split('/')[0] for item in group}) > 1]
    digest = hashlib.sha256(json.dumps(files, sort_keys=True).encode()).hexdigest()
    return {"dataset": str(root), "manifest_sha256": file_hash(root / "manifest.json"), "files_sha256": digest,
            "source_archive_sha256": file_hash(source_archive) if source_archive else None,
            "annotation_semantics": LABELS, "yaml_classes_without_source_annotation": sorted(set(names.values()) - coverage),
            "split_images": dict(counts), "split_boxes": {split: dict(row) for split, row in class_counts.items()},
            "negative_images": dict(negatives), "source_voc_checked": bool(source_archive), "label_issues": issues,
            "input_size_limit": {"max_image_bytes": max_image_bytes, "oversized_images": oversized},
            "near_duplicate_boxes": duplicate_boxes, "exact_cross_split_duplicates": exact_groups,
            "near_cross_split_pairs": sorted(near_pairs, key=lambda item: (item["dhash_distance"], item["left"], item["right"])),
            "scene_grouping": {"available": False, "reason": "manifest contains source filenames but no verified wall segment or capture-session identifiers",
                               "split_rule": "source pixel hash buckets; exact deduplication does not establish scene independence",
                               "near_pair_rule": "64-bit grayscale difference hash Hamming distance <= 4; requires visual review; not a scene identity label"},
            "files": files}


def main():
    parser = argparse.ArgumentParser(description="Audit converted MSD labels and split risks.")
    parser.add_argument("--data-root", type=Path, default=Path("data/yolo/dataset_msd_oriented"))
    parser.add_argument("--source-archive", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--max-image-bytes", type=int, default=10 * 1024 * 1024)
    args = parser.parse_args()
    if args.output.exists():
        parser.error("output already exists")
    result = audit(args.data_root, args.source_archive, args.max_image_bytes)
    payload = json.dumps(result, indent=2, ensure_ascii=False)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("x", encoding="utf-8") as stream:
        stream.write(payload)
    print(json.dumps({key: result[key] for key in ("split_images", "negative_images", "label_issues")}))
    print(f"Cross-split pairs requiring review: {len(result['near_cross_split_pairs'])}")


if __name__ == "__main__":
    main()
