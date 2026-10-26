"""合并 MSD 与青砖标注，保留 MSD 划分，并按来源提供独立测试入口。"""
from __future__ import annotations

import argparse
import hashlib
import io
import json
import math
import shutil
import subprocess
import tempfile
import zipfile
from collections import Counter
from contextlib import ExitStack
from pathlib import Path

from PIL import Image
import yaml

from prepare_dataset import CODES

BRICK_CODES = {"CRACK": "CRACK", "ALKALI": "EFFLORESCENCE", "MISS": "MASONRY_LOSS", "MOSS": "MOSS", "W_E": "W_E"}


def labels(text, names):
    result = []
    for line in text.splitlines():
        if not line.strip():
            continue
        fields = line.split()
        if len(fields) != 5:
            raise ValueError("label must contain class and four coordinates")
        code = names[int(fields[0])]
        x, y, w, h = map(float, fields[1:])
        if not all(math.isfinite(v) for v in (x, y, w, h)) or w <= 0 or h <= 0:
            raise ValueError("invalid bounding box")
        if min(x - w / 2, y - h / 2) < -0.00001 or max(x + w / 2, y + h / 2) > 1.00001:
            raise ValueError("bounding box outside image")
        result.append((code, x, y, w, h))
    return sorted(result)


def file_hash(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def prepare(source: Path, base: Path, output: Path, temp_dir: Path):
    if output.exists():
        raise ValueError("output exists; choose a new dataset directory")
    source_archive = source / "yolo" / "古建筑青砖表面损伤数据集.rar"
    seven_zip = shutil.which("7z") or str(Path("C:/Program Files/7-Zip/7z.exe"))
    if not Path(seven_zip).is_file():
        raise FileNotFoundError("7-Zip is required to read the supplied RAR archive")
    base_manifest = json.loads((base / "manifest.json").read_text(encoding="utf-8"))
    base_names = yaml.safe_load((base / "data.yaml").read_text(encoding="utf-8"))["names"]
    if isinstance(base_names, list):
        base_names = dict(enumerate(base_names))
    records, known = [], set()
    skipped = Counter()
    for item in base_manifest["images"]:
        digest, split = item["sha256"], item["split"]
        if digest in known:
            raise ValueError("duplicate image in base manifest")
        known.add(digest)
        rows = labels((base / "labels" / split / f"{digest}.txt").read_text(encoding="utf-8"), base_names)
        records.append({"id": digest, "dataset": "msd", "split": split, "labels": rows,
                        "source": item["source"], "annotation": item["annotation"],
                        "path": base / "images" / split / f"{digest}.jpg"})
    temp_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="citywall-dataset-", dir=temp_dir) as temporary, ExitStack() as stack:
        archives = {}
        for member in ("label.zip", "train.zip", "valid.zip"):
            destination = Path(temporary) / member
            with destination.open("wb") as stream:
                subprocess.run([seven_zip, "e", "-so", str(source_archive), member], stdout=stream, stderr=subprocess.PIPE, check=True)
            archives[member] = stack.enter_context(zipfile.ZipFile(destination))
        annotated = archives["label.zip"]
        original_codes = [line.strip() for line in annotated.read("label/labels/classes.txt").decode().splitlines() if line.strip()]
        if set(original_codes) != set(BRICK_CODES):
            raise ValueError(f"unexpected source categories: {original_codes}")
        # W_E 保留来源类别码，不推定其与 MSD 的表面溶蚀类等价。
        brick_names = {index: BRICK_CODES[code] for index, code in enumerate(original_codes)}
        source_splits = {}
        for member, prefix in (("train.zip", "train"), ("valid.zip", "valid")):
            archive = archives[member]
            for name in archive.namelist():
                if not name.startswith(prefix + "/images/") or not name.lower().endswith(".jpg"):
                    continue
                filename = Path(name).name
                if filename in source_splits:
                    raise ValueError(f"image occurs in both source splits: {filename}")
                raw = annotated.read("label/images/" + filename)
                if hashlib.sha256(raw).digest() != hashlib.sha256(archive.read(name)).digest():
                    raise ValueError(f"source image versions differ: {filename}")
                annotation = "label/labels/" + Path(filename).stem + ".txt"
                rows = labels(annotated.read(annotation).decode(), brick_names)
                split_rows = labels(archive.read(prefix + "/labels/" + Path(filename).stem + ".txt").decode(), brick_names)
                if rows != split_rows:
                    raise ValueError(f"source annotation versions differ: {filename}")
                source_splits[filename] = prefix
        image_names = [name for name in annotated.namelist() if name.startswith("label/images/") and name.lower().endswith(".jpg")]
        if set(source_splits) != {Path(name).name for name in image_names}:
            raise ValueError("source splits do not cover the annotated images")
        brick, conflicts, excluded = {}, set(), []
        for name in image_names:
            filename = Path(name).name
            raw = annotated.read(name)
            with Image.open(io.BytesIO(raw)) as original:
                image = original.convert("RGB")
                width, height = image.size
                digest = hashlib.sha256(image.tobytes() + f"{width}:{height}".encode()).hexdigest()
            if digest in known:
                skipped["duplicate_with_msd"] += 1
                continue
            annotation = "label/labels/" + Path(filename).stem + ".txt"
            rows = labels(annotated.read(annotation).decode(), brick_names)
            held_out = source_splits[filename] == "valid"
            member = {"source": name, "annotation": annotation, "source_split": source_splits[filename], "labels": rows}
            if digest in brick:
                if brick[digest]["labels"] != rows:
                    conflicts.add(digest)
                brick[digest]["held_out"] |= held_out
                brick[digest]["members"].append(member)
                continue
            brick[digest] = {"id": digest, "dataset": "bluebrick", "source": name,
                             "annotation": annotation, "labels": rows, "held_out": held_out, "members": [member]}
        for item in brick.values():
            if item["id"] in conflicts:
                skipped["conflicting_bluebrick_groups"] += 1
                skipped["conflicting_bluebrick_files"] += len(item["members"])
                excluded.append({"id": item["id"], "reason": "conflicting_annotations", "members": item["members"]})
                continue
            skipped["duplicate_bluebrick"] += len(item["members"]) - 1
            item["split"] = ("val" if int(item["id"][:8], 16) % 2 == 0 else "test") if item["held_out"] else "train"
            records.append(item)
        observed = {row[0] for item in records for row in item["labels"]}
        codes = [code for code in CODES + ["W_E"] if code in observed]
        if set(codes) != observed:
            raise ValueError("unmapped business category")
        output.mkdir(parents=True)
        counts, class_boxes = Counter(), Counter()
        by_source = {}
        manifest_records = []
        for item in records:
            relative = Path(item["split"]) / item["dataset"] / item["id"]
            image_path = output / "images" / relative.with_suffix(".jpg")
            label_path = output / "labels" / relative.with_suffix(".txt")
            image_path.parent.mkdir(parents=True, exist_ok=True)
            label_path.parent.mkdir(parents=True, exist_ok=True)
            if item["dataset"] == "msd":
                shutil.copyfile(item["path"], image_path)
            else:
                with Image.open(io.BytesIO(annotated.read(item["source"]))) as image:
                    image.convert("RGB").save(image_path, quality=95)
            label_path.write_text("\n".join(f"{codes.index(code)} " + " ".join(f"{v:.8f}" for v in box) for code, *box in item["labels"]), encoding="utf-8")
            counts[item["split"]] += 1
            by_source.setdefault(item["dataset"], Counter())[item["split"]] += 1
            class_boxes.update(row[0] for row in item["labels"])
            manifest_records.append({key: item[key] for key in ("id", "dataset", "source", "annotation", "split")} | {"image_sha256": file_hash(image_path)})
        config = {"path": str(output.resolve()), "train": "images/train", "val": "images/val", "test": "images/test", "names": dict(enumerate(codes))}
        (output / "data.yaml").write_text(yaml.safe_dump(config), encoding="utf-8")
        for dataset in by_source:
            subset = config | {split: f"images/{split}/{dataset}" for split in ("train", "val", "test")}
            (output / f"{dataset}.yaml").write_text(yaml.safe_dump(subset), encoding="utf-8")
        manifest = {"counts": dict(counts), "counts_by_source": by_source, "class_boxes": dict(class_boxes), "skipped": dict(skipped),
                    "base_manifest_sha256": file_hash(base / "manifest.json"), "bluebrick_archive_sha256": file_hash(source_archive),
                    "bluebrick_label_mapping": BRICK_CODES, "excluded": excluded, "images": manifest_records}
        (output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
        print(json.dumps({key: manifest[key] for key in ("counts", "counts_by_source", "class_boxes", "skipped")}, ensure_ascii=False))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=Path(r"C:\a\school\毕设"))
    parser.add_argument("--base", type=Path, default=Path("data/yolo/dataset"))
    parser.add_argument("--output", type=Path, default=Path("data/yolo/dataset_combined"))
    parser.add_argument("--temp-dir", type=Path, default=Path("C:/code/tmp"))
    args = parser.parse_args()
    prepare(args.source, args.base, args.output, args.temp_dir)
