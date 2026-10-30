"""按原图划分生成剥落局部训练图片。"""
import argparse
import hashlib
import io
import json
from pathlib import Path

import yaml
from PIL import Image, ImageOps


def bounds(center, image_size, tile_size):
    size = min(image_size, tile_size)
    start = max(0, min(image_size - size, round(center - size / 2)))
    return start, start + size


def labels_in_crop(boxes, crop):
    left, top, right, bottom = crop
    width, height = right - left, bottom - top
    result = []
    for x1, y1, x2, y2 in boxes:
        a, b, c, d = max(x1, left), max(y1, top), min(x2, right), min(y2, bottom)
        if c <= a or d <= b:
            continue
        result.append(f"0 {(a + c - 2 * left) / (2 * width):.8f} {(b + d - 2 * top) / (2 * height):.8f} {(c - a) / width:.8f} {(d - b) / height:.8f}")
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=Path("data/yolo/dataset_cif_spalling"))
    parser.add_argument("--output", type=Path, default=Path("data/yolo/dataset_cif_spalling_tiles"))
    parser.add_argument("--size", type=int, default=1280)
    parser.add_argument("--positive-tiles", type=int, default=3)
    parser.add_argument("--background-tiles", type=int, default=0)
    args = parser.parse_args()
    if args.size < 64 or args.positive_tiles < 1 or args.background_tiles < 0:
        parser.error("invalid crop settings")
    source, output = args.source.resolve(), args.output.resolve()
    if output == source or source in output.parents:
        parser.error("output must not be inside the source dataset")
    source_manifest = source / "manifest.json"
    source_manifest_bytes = source_manifest.read_bytes()
    output.mkdir(parents=True, exist_ok=False)
    seen = {}
    manifest = {"source_manifest_sha256": hashlib.sha256(source_manifest_bytes).hexdigest(), "tile_size": args.size,
                "positive_tiles": args.positive_tiles, "background_tiles": args.background_tiles, "splits": {}}
    for split in ("train", "val"):
        (output / "images" / split).mkdir(parents=True)
        (output / "labels" / split).mkdir(parents=True)
        records = []
        for path in sorted((source / "images" / split).glob("*.jpg")):
            text = (source / "labels" / split / f"{path.stem}.txt").read_text(encoding="utf-8")
            with Image.open(path) as original:
                image = ImageOps.exif_transpose(original).convert("RGB")
            width, height = image.size
            boxes = []
            for line in text.splitlines():
                code, x, y, w, h = map(float, line.split())
                if code != 0 or w <= 0 or h <= 0:
                    raise ValueError(f"invalid source label: {path.name}")
                boxes.append(((x-w/2)*width, (y-h/2)*height, (x+w/2)*width, (y+h/2)*height))
            centers = []
            if boxes:
                count = min(args.positive_tiles, len(boxes))
                indices = [0] if count == 1 else [i * (len(boxes)-1) // (count-1) for i in range(count)]
                centers = [((boxes[i][0]+boxes[i][2])/2, (boxes[i][1]+boxes[i][3])/2) for i in indices]
                if split == "train" and args.background_tiles:
                    candidates = [(width * x / 4, height * y / 4) for y in (1, 2, 3) for x in (1, 2, 3)]
                    candidates.sort(key=lambda point: hashlib.sha256(f"{path.stem}:{point}".encode()).digest())
                    background_crops = set()
                    for x, y in candidates:
                        left, right = bounds(x, width, args.size)
                        top, bottom = bounds(y, height, args.size)
                        crop = (left, top, right, bottom)
                        if crop not in background_crops and not labels_in_crop(boxes, crop):
                            background_crops.add(crop)
                            centers.append((x, y))
                            if len(background_crops) >= args.background_tiles:
                                break
            else:
                centers = [(width/2, height/2)]
            crops = set()
            for index, (x, y) in enumerate(centers):
                left, right = bounds(x, width, args.size)
                top, bottom = bounds(y, height, args.size)
                crop = (left, top, right, bottom)
                if crop in crops:
                    continue
                crops.add(crop)
                labels = labels_in_crop(boxes, crop)
                if boxes and index < count and not labels:
                    raise ValueError(f"positive crop has no labels: {path.name}")
                buffer = io.BytesIO()
                image.crop(crop).save(buffer, format="JPEG", quality=95)
                content = buffer.getvalue()
                digest = hashlib.sha256(content).hexdigest()
                label_text = "\n".join(labels) + ("\n" if labels else "")
                if digest in seen:
                    previous_split, previous_labels = seen[digest]
                    if previous_split != split or previous_labels != label_text:
                        raise ValueError("duplicate crop across splits or with conflicting labels")
                    continue
                seen[digest] = (split, label_text)
                (output / "images" / split / f"{digest}.jpg").write_bytes(content)
                (output / "labels" / split / f"{digest}.txt").write_text(label_text, encoding="utf-8")
                records.append({"sha256": digest, "source_image": path.name, "crop": list(crop), "boxes": len(labels)})
        manifest["splits"][split] = records
        print(f"{split}: {len(records)} crops", flush=True)
    (output / "data.yaml").write_text(yaml.safe_dump({"path": str(output), "train": "images/train", "val": "images/val", "names": {0: "SURFACE_SPALLING"}}, sort_keys=False), encoding="utf-8")
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")


if __name__ == "__main__":
    main()
