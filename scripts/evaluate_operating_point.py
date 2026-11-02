"""复用部署检测链路评估固定运行点及人工复核负担，不代替标准 mAP。"""
import argparse
from collections import Counter
from dataclasses import asdict, replace
import hashlib
import io
import json
import math
import os
from pathlib import Path
import platform
import statistics
import sys
import time

REPOSITORY = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPOSITORY / "recognizer"))


def file_hash(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def overlap(a, b):
    intersection = max(0, min(a[2], b[2]) - max(a[0], b[0])) * max(0, min(a[3], b[3]) - max(a[1], b[1]))
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - intersection
    return intersection / union if union > 0 else 0


def match_details(truth, predictions, threshold):
    matched, counts, assignments = set(), {}, []
    for prediction_index, (code, score, box) in sorted(enumerate(predictions), key=lambda item: -item[1][1]):
        row = counts.setdefault(code, {"tp": 0, "fp": 0, "fn": 0})
        candidates = [(overlap(box, target_box), index) for index, (target, target_box) in enumerate(truth)
                      if target == code and index not in matched]
        best = max(candidates, default=(0, -1))
        hit = best[0] >= threshold and best[1] >= 0
        row["tp" if hit else "fp"] += 1
        if hit:
            matched.add(best[1])
        assignments.append({"prediction_index": prediction_index, "truth_index": best[1] if hit else None,
                            "iou": best[0]})
    for index, (code, box) in enumerate(truth):
        row = counts.setdefault(code, {"tp": 0, "fp": 0, "fn": 0})
        if index not in matched:
            row["fn"] += 1
    return counts, assignments, sorted(set(range(len(truth))) - matched)


def match(truth, predictions, threshold):
    return match_details(truth, predictions, threshold)[0]


def metrics(counts):
    tp, fp, fn = (counts[key] for key in ("tp", "fp", "fn"))
    precision = tp / (tp + fp) if tp + fp else 0
    recall = tp / (tp + fn) if tp + fn else 0
    return {**counts, "precision": precision, "recall": recall,
            "f1": 2 * tp / (2 * tp + fp + fn) if 2 * tp + fp + fn else 0}


def read_truth(content, names, width, height, annotated_classes):
    truth = []
    for line in content.decode("utf-8").splitlines():
        if not line.strip():
            continue
        fields = line.split()
        if len(fields) != 5:
            raise ValueError("labels must contain class index and four normalized coordinates")
        index, x, y, w, h = map(float, fields)
        if not all(math.isfinite(value) for value in (index, x, y, w, h)) or not index.is_integer() or int(index) not in names:
            raise ValueError("invalid label class or coordinates")
        code = names[int(index)]
        if code not in annotated_classes:
            raise ValueError(f"label class {code} is outside declared annotation coverage")
        box = [x - w/2, y - h/2, x + w/2, y + h/2]
        if w <= 0 or h <= 0 or min(box) < -1e-6 or max(box) > 1 + 1e-6:
            raise ValueError("label box is outside the normalized image")
        truth.append((code, [max(0, box[0]) * width, max(0, box[1]) * height,
                             min(1, box[2]) * width, min(1, box[3]) * height]))
    return truth


def score_image(truth, detections, annotated_classes, threshold):
    predictions = [(item.class_code, item.confidence, [item.bbox.x1, item.bbox.y1, item.bbox.x2, item.bbox.y2])
                   for item in detections]
    indices = [index for index, item in enumerate(predictions) if item[0] in annotated_classes]
    counts, assignments, missed = match_details(truth, [predictions[index] for index in indices], threshold)
    for assignment in assignments:
        assignment["prediction_index"] = indices[assignment["prediction_index"]]
    ignored = Counter(code for code, _, _ in predictions if code not in annotated_classes)
    return {"counts": counts, "matches": assignments, "missed_truth_indices": missed,
            "evaluated_predictions": len(indices), "unassessed_predictions": dict(sorted(ignored.items()))}


class TimedDetector:
    def __init__(self, detector):
        self.detector = detector
        self.elapsed_ms = 0

    def detect(self, image_bytes):
        started = time.perf_counter()
        output = self.detector.detect(image_bytes)
        self.elapsed_ms = (time.perf_counter() - started) * 1000
        return output


def detector_members(detector):
    from app.models.routed import ClassRoutedDetector
    from app.models.tiled import TiledDetector
    if isinstance(detector, ClassRoutedDetector):
        return detector_members(detector.primary) + detector_members(detector.specialist)
    if isinstance(detector, TiledDetector):
        return detector_members(detector.detector)
    return [detector]


def runtime_member(detector):
    model = detector._model
    predictor = model.predictor
    keys = ("imgsz", "conf", "iou", "max_det", "agnostic_nms", "classes", "rect", "half", "augment",
            "device", "batch", "dnn", "compile", "quantize", "end2end")
    return {"weights": str(Path(detector.weights_path).resolve()), "classes": model.names,
            "parameters": {key: getattr(predictor.args, key) for key in keys if hasattr(predictor.args, key)},
            "actual_device": str(predictor.device)}


def timing_summary(values):
    return {"mean_ms": statistics.mean(values), "median_ms": statistics.median(values),
            "min_ms": min(values), "max_ms": max(values),
            "p95_ms": sorted(values)[math.ceil(len(values) * .95) - 1]}


def evaluate(data_path, split, config, annotated_classes, match_iou=.5, warmup=1):
    import torch
    import ultralytics
    import yaml
    from PIL import Image
    from app.models.factory import build_detector
    from app.models.yolo import CLASS_CODES, resolve_weights_path
    from app.service import RecognitionService

    data_path = data_path.resolve()
    data_bytes = data_path.read_bytes()
    data = yaml.safe_load(data_bytes)
    names = data["names"]
    names = dict(enumerate(names)) if isinstance(names, list) else {int(key): value for key, value in names.items()}
    annotated_classes = frozenset(annotated_classes)
    if not annotated_classes or not annotated_classes <= set(names.values()) or not set(names.values()) <= CLASS_CODES:
        raise ValueError("annotation coverage and dataset names must use explicit business category codes")
    if len(set(names.values())) != len(names):
        raise ValueError("dataset class names must be unique")
    root = Path(data.get("path", data_path.parent))
    root = (data_path.parent / root).resolve() if not root.is_absolute() else root.resolve()
    image_root = root / "images"
    split_root = root / data[split]
    if not split_root.is_dir():
        raise ValueError("split must point to an images directory")
    paths = sorted(path for path in split_root.rglob("*") if path.suffix.lower() in {".jpg", ".jpeg", ".png", ".webp"})
    if not paths:
        raise ValueError("no dataset images found for this split")
    if config.detector_mode != "yolo":
        raise ValueError("evaluation requires yolo mode, not fixed development responses")
    config = replace(config, **{key: str(resolve_weights_path(getattr(config, key)).resolve())
                               for key in ("weights_path", "specialist_weights_path", "spalling_weights_path") if getattr(config, key)})
    detector = build_detector(config)
    members = detector_members(detector)
    weights = [{"path": member.weights_path, "sha256": file_hash(Path(member.weights_path))} for member in members]
    timer = TimedDetector(detector)
    service = RecognitionService(timer, config.model_version)
    warmup_ms = []
    for _ in range(warmup):
        content = paths[0].read_bytes()
        if len(content) > config.max_image_bytes:
            raise ValueError(f"image exceeds service size limit: {paths[0]}")
        started = time.perf_counter()
        service.recognize("warmup", content, config.allowed_formats)
        warmup_ms.append((time.perf_counter() - started) * 1000)
    digest = hashlib.sha256(json.dumps(names, sort_keys=True).encode())
    totals = {code: {"tp": 0, "fp": 0, "fn": 0} for code in sorted(annotated_classes)}
    cases = []
    for path in paths:
        relative = path.relative_to(image_root)
        label_path = (root / "labels" / relative).with_suffix(".txt")
        content, label_bytes = path.read_bytes(), label_path.read_bytes()
        if len(content) > config.max_image_bytes:
            raise ValueError(f"image exceeds service size limit: {path}")
        image_sha256, label_sha256 = hashlib.sha256(content).hexdigest(), hashlib.sha256(label_bytes).hexdigest()
        digest.update(relative.as_posix().encode())
        digest.update(bytes.fromhex(image_sha256))
        digest.update(bytes.fromhex(label_sha256))
        started = time.perf_counter()
        result = service.recognize(relative.as_posix(), content, config.allowed_formats)
        recognition_ms = (time.perf_counter() - started) * 1000
        width, height = result.image_width, result.image_height
        try:
            truth = read_truth(label_bytes, names, width, height, annotated_classes)
        except ValueError as error:
            raise ValueError(f"{label_path}: {error}") from error
        scored = score_image(truth, result.detections, annotated_classes, match_iou)
        for code, row in scored["counts"].items():
            for key, value in row.items():
                totals[code][key] += value
        with Image.open(io.BytesIO(content)) as image:
            orientation = image.getexif().get(274, 1)
        cases.append({"image": relative.as_posix(), "image_sha256": image_sha256, "label_sha256": label_sha256,
                      "width": width, "height": height, "source_exif_orientation": orientation,
                      "truth": [{"class_code": code, "bbox": box} for code, box in truth],
                      "predictions": [item.model_dump() for item in result.detections], **scored,
                      "detector_ms": timer.elapsed_ms, "recognition_without_vlm_ms": recognition_ms})
        print(f"{len(cases)}/{len(paths)} {relative.as_posix()}: {len(result.detections)} candidates", flush=True)
    for member in weights:
        if file_hash(Path(member["path"])) != member["sha256"]:
            raise ValueError("weights changed during evaluation")
    aggregate = {key: sum(row[key] for row in totals.values()) for key in ("tp", "fp", "fn")}
    ignored = Counter()
    for case in cases:
        ignored.update(case["unassessed_predictions"])
    source_paths = [Path(__file__), *(REPOSITORY / "recognizer" / "app").rglob("*.py")]
    source_hashes = {path.relative_to(REPOSITORY).as_posix(): file_hash(path) for path in sorted(source_paths)}
    candidates = [len(case["predictions"]) for case in cases]
    return {"dataset": str(data_path), "data_yaml_sha256": hashlib.sha256(data_bytes).hexdigest(), "split": split, "images": len(paths),
            "dataset_sha256": digest.hexdigest(),
            "source_manifest_sha256": file_hash(root / "manifest.json") if (root / "manifest.json").is_file() else None,
            "annotation_coverage": {"classes": sorted(annotated_classes), "scope": "exhaustive for declared classes on every selected image",
                                    "outside_scope": "retained as unassessed candidates; excluded from TP/FP/FN"},
            "configuration": asdict(config), "weights": weights, "runtime_members": [runtime_member(member) for member in members],
            "source_sha256": source_hashes,
            "environment": {"python": platform.python_version(), "platform": platform.platform(), "processor": platform.processor(),
                            "torch": torch.__version__, "ultralytics": ultralytics.__version__, "pillow": Image.__version__,
                            "torch_threads": torch.get_num_threads(), "cuda": torch.version.cuda,
                            "cuda_devices": [torch.cuda.get_device_name(index) for index in range(torch.cuda.device_count())]},
            "match_iou": match_iou,
            "matching": "descending service confidence (stable ties); highest IoU unmatched same-class truth; ties choose higher truth index; IoU >= threshold",
            "coordinates": "EXIF-normalized pixels; service-rounded/clipped integer predictions and 4-decimal confidence; floating-point label boxes",
            "aggregate": metrics(aggregate), "per_class": {code: metrics(row) for code, row in totals.items()},
            "candidate_burden": {"total": sum(candidates), "mean_per_image": statistics.mean(candidates),
                                 "max_per_image": max(candidates), "unassessed_by_class": dict(sorted(ignored.items()))},
            "timing": {"warmup_recognition_ms": warmup_ms, "warmup_images_counted_in_metrics": False,
                       "detector": timing_summary([case["detector_ms"] for case in cases]),
                       "recognition_without_vlm": timing_summary([case["recognition_without_vlm_ms"] for case in cases]),
                       "scope": "synchronous wall time; detector includes decode, routes, tiles and postprocessing; recognition adds image validation; excludes file IO, HTTP and VLM"},
            "per_image": cases}


def main():
    from app.config import load_settings
    parser = argparse.ArgumentParser(description="Evaluate deployed detector candidates at a fixed operating point.")
    parser.add_argument("--data", type=Path, required=True)
    parser.add_argument("--config", type=Path, default=Path(os.getenv("RECOGNIZER_CONFIG", str(REPOSITORY / "conf/recognizer.json"))))
    parser.add_argument("--weights", type=Path, help="single-weight control; disables all specialist routing and tiling")
    parser.add_argument("--annotated-classes", nargs="+", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--split", choices=("val", "test"), default="val")
    parser.add_argument("--device")
    parser.add_argument("--confidence", type=float)
    parser.add_argument("--nms-iou", type=float)
    parser.add_argument("--match-iou", type=float, default=.5)
    parser.add_argument("--imgsz", type=int)
    parser.add_argument("--warmup", type=int, default=1)
    args = parser.parse_args()
    if args.output.exists():
        parser.error("output already exists")
    if any(value is not None and not 0 < value <= 1 for value in (args.confidence, args.nms_iou, args.match_iou)):
        parser.error("confidence and IoU thresholds must be in (0, 1]")
    if args.warmup < 0 or (args.imgsz is not None and args.imgsz < 32):
        parser.error("warmup must be nonnegative and imgsz must be at least 32")
    config_sha256 = file_hash(args.config)
    config = load_settings(args.config).recognition
    overrides = {key: value for key, value in {"device": args.device, "confidence_threshold": args.confidence,
                 "nms_iou": args.nms_iou, "imgsz": args.imgsz}.items() if value is not None}
    if args.weights:
        overrides.update(weights_path=str(args.weights.resolve()), model_version="single_weight",
                         specialist_weights_path="", specialist_class_codes=[], spalling_weights_path="", spalling_tile_size=0)
    result = evaluate(args.data, args.split, replace(config, **overrides), args.annotated_classes, args.match_iou, args.warmup)
    result.update(mode="single" if args.weights else "deployment", config_file=str(args.config.resolve()),
                  config_file_sha256=config_sha256, cli_overrides=overrides)
    payload = json.dumps(result, indent=2, ensure_ascii=False, allow_nan=False)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("x", encoding="utf-8") as stream:
        stream.write(payload)
    print(json.dumps(result["aggregate"]))


if __name__ == "__main__":
    main()
