"""从 CIF 数据集准备剥落目标检测训练集。"""
import argparse
import hashlib
import io
import json
import math
from pathlib import Path
from urllib.parse import urlparse

import requests
import yaml
from PIL import Image, ImageOps
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry


DATASET = "ibm-research/cif-dataset"
REVISION = "75057c6b0838a24c401c292a7e54a88a4cf28271"
SOURCE = "https://huggingface.co/datasets/ibm-research/cif-dataset"
LICENSE = "CDLA-Permissive-2.0"


def prepare_split(session, output, split, positive_limit, negative_limit, seen):
    counts = {"positive": 0, "negative": 0, "boxes": 0}
    records = []
    offset = 0
    source_split = split + "_full"
    while counts["positive"] < positive_limit or counts["negative"] < negative_limit:
        response = session.get("https://datasets-server.huggingface.co/rows", params={
            "dataset": DATASET, "config": "default", "split": source_split, "offset": offset, "length": 100,
        }, timeout=60)
        response.raise_for_status()
        rows = response.json()["rows"]
        if not rows:
            raise ValueError(f"not enough usable images in {source_split}: {counts}")
        offset += len(rows)
        for entry in rows:
            row = entry["row"]
            objects = row["objects"]
            boxes = [box for category, box in zip(objects["category_id"], objects["bbox"]) if category == 6]
            group = "positive" if boxes else "negative"
            limit = positive_limit if boxes else negative_limit
            if counts[group] >= limit:
                continue
            if len(objects["category_id"]) != len(objects["bbox"]) or len(objects["iscrowd"]) != len(objects["bbox"]):
                raise ValueError("inconsistent object annotation")
            if any(category == 6 and crowd for category, crowd in zip(objects["category_id"], objects["iscrowd"])):
                continue
            width, height = row["width"], row["height"]
            if not width or not height:
                continue
            labels = []
            for x, y, w, h in boxes:
                if not all(math.isfinite(value) for value in (x, y, w, h)) or x < 0 or y < 0 or w <= 0 or h <= 0 or x + w > width + .01 or y + h > height + .01:
                    break
                w, h = min(w, width - x), min(h, height - y)
                if w <= 0 or h <= 0:
                    break
                labels.append(f"0 {(x + w/2)/width:.8f} {(y + h/2)/height:.8f} {w/width:.8f} {h/height:.8f}")
            else:
                url = row["image"]["src"]
                if f"/--/{REVISION}/--/" not in urlparse(url).path:
                    raise ValueError("unexpected dataset revision")
                image_response = session.get(url, timeout=90)
                image_response.raise_for_status()
                content = image_response.content
                with Image.open(io.BytesIO(content)) as image:
                    if image.format != "JPEG" or image.getexif().get(274, 1) != 1 or ImageOps.exif_transpose(image).size != (width, height):
                        raise ValueError(f"image dimensions mismatch: {row['image_id']}")
                digest = hashlib.sha256(content).hexdigest()
                if digest in seen:
                    continue
                seen.add(digest)
                (output / "images" / split / f"{digest}.jpg").write_bytes(content)
                (output / "labels" / split / f"{digest}.txt").write_text("\n".join(labels) + ("\n" if labels else ""), encoding="utf-8")
                counts[group] += 1
                counts["boxes"] += len(labels)
                records.append({"image_id": row["image_id"], "file_name": row["file_name"], "sha256": digest, "boxes": len(labels)})
                print(f"{split}: {counts}", flush=True)
            if counts["positive"] >= positive_limit and counts["negative"] >= negative_limit:
                break
    return {"counts": counts, "images": records}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("data/yolo/dataset_cif_spalling"))
    parser.add_argument("--train-positive", type=int, default=160)
    parser.add_argument("--train-negative", type=int, default=40)
    parser.add_argument("--val-positive", type=int, default=40)
    parser.add_argument("--val-negative", type=int, default=10)
    args = parser.parse_args()
    if min(args.train_positive, args.train_negative, args.val_positive, args.val_negative) < 1:
        parser.error("each split needs positive and negative images")
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    for split in ("train", "val"):
        (output / "images" / split).mkdir(parents=True)
        (output / "labels" / split).mkdir(parents=True)
    seen = set()
    with requests.Session() as session:
        session.mount("https://", HTTPAdapter(max_retries=Retry(total=3, backoff_factor=.5, status_forcelist=(429, 500, 502, 503, 504))))
        train = prepare_split(session, output, "train", args.train_positive, args.train_negative, seen)
        val = prepare_split(session, output, "val", args.val_positive, args.val_negative, seen)
    (output / "data.yaml").write_text(yaml.safe_dump({"path": str(output), "train": "images/train", "val": "images/val", "names": {0: "SURFACE_SPALLING"}}, sort_keys=False), encoding="utf-8")
    (output / "manifest.json").write_text(json.dumps({"source": SOURCE, "revision": REVISION, "license": LICENSE, "class": "Spalling", "train": train, "val": val}, indent=2), encoding="utf-8")


if __name__ == "__main__":
    main()
