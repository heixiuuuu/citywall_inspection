"""按固定置信度评估转换后数据集的候选负担，不代替 mAP 评估。"""
import argparse
import hashlib
import json
from pathlib import Path


def overlap(a, b):
    intersection = max(0, min(a[2], b[2]) - max(a[0], b[0])) * max(0, min(a[3], b[3]) - max(a[1], b[1]))
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - intersection
    return intersection / union if union > 0 else 0


def match(truth, predictions, threshold):
    matched = set()
    counts = {}
    for code, score, box in sorted(predictions, key=lambda item: -item[1]):
        row = counts.setdefault(code, {"tp": 0, "fp": 0, "fn": 0})
        candidates = [(overlap(box, target_box), index) for index, (target, target_box) in enumerate(truth)
                      if target == code and index not in matched]
        best = max(candidates, default=(0, -1))
        if best[0] >= threshold:
            row["tp"] += 1
            matched.add(best[1])
        else:
            row["fp"] += 1
    for index, (code, box) in enumerate(truth):
        row = counts.setdefault(code, {"tp": 0, "fp": 0, "fn": 0})
        if index not in matched:
            row["fn"] += 1
    return counts


def metrics(counts):
    tp, fp, fn = (counts[key] for key in ("tp", "fp", "fn"))
    precision = tp / (tp + fp) if tp + fp else 0
    recall = tp / (tp + fn) if tp + fn else 0
    return {**counts, "precision": precision, "recall": recall,
            "f1": 2 * tp / (2 * tp + fp + fn) if 2 * tp + fp + fn else 0}


def main():
    import yaml
    import ultralytics
    from ultralytics import YOLO
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", type=Path, required=True)
    parser.add_argument("--weights", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--split", choices=("val", "test"), default="val")
    parser.add_argument("--device", default="cpu")
    parser.add_argument("--confidence", type=float, default=0.35)
    parser.add_argument("--nms-iou", type=float, default=0.7)
    parser.add_argument("--match-iou", type=float, default=0.5)
    parser.add_argument("--imgsz", type=int)
    args = parser.parse_args()
    if args.output.exists():
        parser.error("output already exists")
    if any(not 0 < value <= 1 for value in (args.confidence, args.nms_iou, args.match_iou)):
        parser.error("confidence and IoU thresholds must be in (0, 1]")
    data_path = args.data.resolve()
    data = yaml.safe_load(data_path.read_text(encoding="utf-8"))
    root = Path(data.get("path", data_path.parent))
    if not root.is_absolute():
        root = data_path.parent / root
    root = root.resolve()
    image_root = root / "images"
    split_root = root / data[args.split]
    paths = sorted(path for path in split_root.rglob("*") if path.suffix.lower() in {".jpg", ".jpeg", ".png", ".webp"})
    if not paths:
        parser.error("no converted dataset images found for this split")
    names = data["names"]
    model = YOLO(str(args.weights.resolve()))
    image_size = args.imgsz or model.overrides.get("imgsz", 640)
    digest = hashlib.sha256(json.dumps(names, sort_keys=True).encode())
    totals = {}
    cases = []
    results = model.predict([str(path) for path in paths], imgsz=image_size, conf=args.confidence,
                            iou=args.nms_iou, device=args.device, stream=True, verbose=False)
    for path, result in zip(paths, results):
        relative = path.relative_to(image_root)
        label_path = (root / "labels" / relative).with_suffix(".txt")
        label_bytes = label_path.read_bytes()
        digest.update(relative.as_posix().encode())
        digest.update(hashlib.sha256(path.read_bytes()).digest())
        digest.update(hashlib.sha256(label_bytes).digest())
        height, width = result.orig_shape
        truth = []
        for line in label_bytes.decode().splitlines():
            code, x, y, w, h = map(float, line.split())
            truth.append((names[int(code)], [(x-w/2)*width, (y-h/2)*height, (x+w/2)*width, (y+h/2)*height]))
        boxes = result.boxes
        predictions = [(model.names[int(code)], float(score), box) for code, score, box in
                       zip(boxes.cls.tolist(), boxes.conf.tolist(), boxes.xyxy.tolist())]
        counts = match(truth, predictions, args.match_iou)
        for code, row in counts.items():
            total = totals.setdefault(code, {"tp": 0, "fp": 0, "fn": 0})
            for key, value in row.items():
                total[key] += value
        cases.append({"image": relative.as_posix(), "truth": len(truth), "predictions": len(predictions),
                      **{key: sum(row[key] for row in counts.values()) for key in ("tp", "fp", "fn")}})
    aggregate = {key: sum(row[key] for row in totals.values()) for key in ("tp", "fp", "fn")}
    result = {"dataset": str(data_path), "split": args.split, "images": len(paths), "dataset_sha256": digest.hexdigest(),
              "weights": str(args.weights.resolve()), "weights_sha256": hashlib.sha256(args.weights.read_bytes()).hexdigest(),
              "ultralytics": ultralytics.__version__, "device": args.device, "imgsz": image_size,
              "confidence": args.confidence, "nms_iou": args.nms_iou, "match_iou": args.match_iou,
              "matching": "descending prediction confidence; best IoU unmatched truth of the same semantic class",
              "aggregate": metrics(aggregate), "per_class": {code: metrics(row) for code, row in sorted(totals.items())},
              "per_image": cases}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2), encoding="utf-8")
    print(json.dumps(result["aggregate"]))


if __name__ == "__main__":
    main()
