"""在验证集服务候选上比较有限阈值与路由实验，并报告 AP 和错误分布。"""
import argparse
from collections import Counter
import copy
import json
from pathlib import Path
from types import SimpleNamespace
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from scripts.evaluate_operating_point import file_hash, metrics, overlap, score_image
from app.schemas import Detection


def coordinates(box):
    return [box[key] for key in ("x1", "y1", "x2", "y2")]


def reroute(primary, combined, specialist_classes):
    if (primary["dataset_sha256"] != combined["dataset_sha256"] or primary["weights"][0] != combined["weights"][0]
            or primary.get("input_dataset_sha256") != combined.get("input_dataset_sha256")):
        raise ValueError("routing comparison requires identical dataset and primary weights")
    if primary["runtime_members"][0] != combined["runtime_members"][0] or primary["annotation_coverage"] != combined["annotation_coverage"]:
        raise ValueError("routing comparison requires identical primary inference parameters and annotation coverage")
    result = copy.deepcopy(combined)
    specialists = set(specialist_classes)
    primary_cases = {case["image"]: case for case in primary["per_image"]}
    for case in result["per_image"]:
        base = primary_cases[case["image"]]
        if case["truth"] != base["truth"] or case["image_sha256"] != base["image_sha256"]:
            raise ValueError("comparison images or labels differ")
        case["predictions"] = ([item for item in base["predictions"] if item["class_code"] not in specialists | {"SURFACE_SPALLING"}]
                               + [item for item in case["predictions"] if item["class_code"] in specialists | {"SURFACE_SPALLING"}])
        for index, item in enumerate(case["predictions"]):
            item["detection_id"] = f"d{index:04d}"
    result["configuration"]["specialist_class_codes"] = sorted(specialists)
    return result


def operating_point(report, threshold):
    codes = set(report["annotation_coverage"]["classes"])
    totals = {code: {"tp": 0, "fp": 0, "fn": 0} for code in sorted(codes)}
    errors, burden, ignored, examples = Counter(), [], Counter(), []
    for case in report["per_image"]:
        predictions = [item for item in case["predictions"]
                       if item["class_code"] == "SURFACE_SPALLING" or item["confidence"] >= threshold]
        truth = [(item["class_code"], item["bbox"]) for item in case["truth"]]
        scored = score_image(truth, [Detection(**item) for item in predictions], codes, report["match_iou"])
        burden.append(len(predictions))
        ignored.update(scored["unassessed_predictions"])
        for code, row in scored["counts"].items():
            for key, value in row.items():
                totals[code][key] += value
        for index in scored["missed_truth_indices"]:
            code, box = truth[index]
            same = [overlap(box, coordinates(item["bbox"])) for item in predictions if item["class_code"] == code]
            other = [overlap(box, coordinates(item["bbox"])) for item in predictions if item["class_code"] in codes - {code}]
            best = max(same, default=0)
            category = "assignment_competition" if best >= .5 else "localization" if best >= .1 else "class_confusion" if max(other, default=0) >= .5 else "no_overlapping_candidate"
            errors["fn_" + category] += 1
            area = (box[2]-box[0])*(box[3]-box[1])/(case["width"]*case["height"])
            errors["fn_small_under_1pct" if area < .01 else "fn_other_size"] += 1
            if not any(item["kind"] == "fn_" + category for item in examples):
                examples.append({"kind": "fn_" + category, "image": case["image"], "truth_index": index})
        for assignment in scored["matches"]:
            if assignment["truth_index"] is not None:
                continue
            item = predictions[assignment["prediction_index"]]
            box = coordinates(item["bbox"])
            same = max((overlap(box, target) for code, target in truth if code == item["class_code"]), default=0)
            other = max((overlap(box, target) for code, target in truth if code != item["class_code"]), default=0)
            category = "duplicate_or_competition" if same >= .5 else "localization" if same >= .1 else "class_confusion" if other >= .5 else "background_or_incomplete_annotation"
            errors["fp_" + category] += 1
            if not any(example["kind"] == "fp_" + category for example in examples):
                examples.append({"kind": "fp_" + category, "image": case["image"], "prediction": item})
    aggregate = {key: sum(row[key] for row in totals.values()) for key in ("tp", "fp", "fn")}
    return {"threshold": threshold, "aggregate": metrics(aggregate), "per_class": {code: metrics(row) for code, row in totals.items()},
            "candidate_burden": {"total": sum(burden), "mean_per_image": sum(burden)/len(burden), "max_per_image": max(burden),
                                 "unassessed_by_class": dict(ignored)},
            "geometric_error_categories": dict(errors), "failure_examples": examples}


def average_precision(report):
    import numpy as np
    import torch
    import ultralytics
    from ultralytics.models.yolo.detect.val import DetectionValidator
    from ultralytics.utils.metrics import ap_per_class, box_iou
    codes = sorted(report["annotation_coverage"]["classes"])
    mapping = {code: index for index, code in enumerate(codes)}
    for member in report["runtime_members"]:
        if set(member["classes"].values()) & set(codes) and member["parameters"]["conf"] > .001:
            raise ValueError("AP requires ranking inference confidence <= 0.001 for every assessed member")
    matcher = SimpleNamespace(iouv=torch.linspace(.5, .95, 10))
    correct, scores, classes, targets = [], [], [], []
    for case in report["per_image"]:
        predictions = sorted((item for item in case["predictions"] if item["class_code"] in mapping), key=lambda item: -item["confidence"])
        prediction_classes = torch.tensor([mapping[item["class_code"]] for item in predictions], dtype=torch.int64)
        truth_classes = torch.tensor([mapping[item["class_code"]] for item in case["truth"]], dtype=torch.int64)
        pred_boxes = torch.tensor([coordinates(item["bbox"]) for item in predictions], dtype=torch.float32).reshape(-1, 4)
        truth_boxes = torch.tensor([item["bbox"] for item in case["truth"]], dtype=torch.float32).reshape(-1, 4)
        matrix = (DetectionValidator.match_predictions(matcher, prediction_classes, truth_classes, box_iou(truth_boxes, pred_boxes))
                  if len(truth_classes) and len(prediction_classes) else torch.zeros((len(predictions), 10), dtype=torch.bool))
        correct.append(matrix.numpy())
        scores.extend(item["confidence"] for item in predictions)
        classes.extend(prediction_classes.tolist())
        targets.extend(truth_classes.tolist())
    result = ap_per_class(np.concatenate(correct), np.asarray(scores), np.asarray(classes), np.asarray(targets),
                          names=dict(enumerate(codes)), plot=False)
    ap, class_indices = result[5], result[6]
    per_class = {code: {"map50": None, "map50_95": None} for code in codes}
    for row, index in enumerate(class_indices):
        per_class[codes[index]] = {"map50": float(ap[row, 0]), "map50_95": float(ap[row].mean())}
    return {"map50": float(ap[:, 0].mean()) if len(ap) else None,
            "map50_95": float(ap.mean()) if len(ap) else None, "per_class": per_class,
            "protocol": "Ultralytics DetectionValidator.match_predictions and ap_per_class at IoU 0.50:0.05:0.95; post-service boxes and scores; macro over classes with ground truth",
            "implementation": {"ultralytics": ultralytics.__version__, "numpy": np.__version__, "torch": torch.__version__},
            "ranking_confidence_max": .001}


def compare(primary, combined):
    if primary["split"] != "val" or combined["split"] != "val":
        raise ValueError("parameter selection accepts validation reports only")
    routed = reroute(primary, combined, ["CRACK", "MOSS"])
    rows = []
    for name, report, thresholds in (("primary_control", primary, [.35]), ("current_routes", combined, [.35, .2]),
                                      ("efflorescence_to_primary", routed, [.35, .2])):
        ap = average_precision(report)
        for threshold in thresholds:
            row = operating_point(report, threshold)
            row.update(name=f"{name}_{threshold}", specialist_classes=report["configuration"]["specialist_class_codes"], ap=ap)
            rows.append(row)
    choices = [row for row in rows if not row["name"].startswith("primary_control")]
    selected = max(choices, key=lambda row: (row["aggregate"]["f1"], -row["candidate_burden"]["total"]))
    configuration = copy.deepcopy(combined["configuration"])
    configuration.update(confidence_threshold=selected["threshold"], specialist_class_codes=selected["specialist_classes"],
                         model_version="yolo_routed_msd_candidate")
    return {"dataset_sha256": combined["dataset_sha256"], "weights": combined["weights"], "split": "val",
            "selection_rule": "maximum validation micro F1; ties prefer fewer total review candidates; two factors only: confidence 0.35/0.20 and efflorescence route",
            "cache_filter_caveat": "threshold filtering uses 4-decimal service confidence; selected point must be rerun through the actual service before freezing",
            "experiments": rows, "selected": selected["name"], "candidate_configuration": configuration}


def summarize_frozen_test(primary, combined, frozen):
    if primary["split"] != "test" or combined["split"] != "test":
        raise ValueError("frozen test summary requires test reports")
    if combined["weights"] != frozen["weights"]:
        raise ValueError("test weights differ from frozen selection")
    config = frozen["candidate_configuration"]
    for key, value in config.items():
        if key not in {"confidence_threshold", "specialist_class_codes", "model_version"} and combined["configuration"].get(key) != value:
            raise ValueError(f"test inference configuration differs from frozen selection: {key}")
    routed = reroute(primary, combined, config["specialist_class_codes"])
    rows = []
    for name, report, threshold in (("primary_control", primary, frozen["baseline_confidence"]),
                                    ("current_deployment", combined, frozen["baseline_confidence"]),
                                    ("frozen_candidate", routed, config["confidence_threshold"])):
        row = operating_point(report, threshold)
        row.update(name=name, ap=average_precision(report))
        rows.append(row)
    return {"split": "test", "dataset_sha256": combined["dataset_sha256"], "weights": combined["weights"],
            "selected": frozen["selected"], "experiments": rows,
            "input_admission": combined.get("input_admission"),
            "test_policy": frozen["test_policy"], "selection_performed": False}


def attach_operating_results(summary, primary, combined, frozen, reports):
    expected = {"primary_control": copy.deepcopy(primary["configuration"]),
                "current_deployment": copy.deepcopy(combined["configuration"]),
                "frozen_candidate": frozen["candidate_configuration"]}
    for name in ("primary_control", "current_deployment"):
        expected[name]["confidence_threshold"] = frozen["baseline_confidence"]
    for row in summary["experiments"]:
        actual = reports[row["name"]]
        if (actual["split"] != "test" or actual["dataset_sha256"] != summary["dataset_sha256"] or actual["configuration"] != expected[row["name"]]
                or actual.get("input_dataset_sha256") != combined.get("input_dataset_sha256")):
            raise ValueError("actual operating-point data or configuration differs from frozen experiment")
        expected_weights = primary["weights"] if row["name"] == "primary_control" else frozen["weights"]
        if actual["weights"] != expected_weights:
            raise ValueError("actual operating-point weights differ from frozen experiment")
        row["cache_metrics_match_actual"] = row["aggregate"] == actual["aggregate"] and row["candidate_burden"] == actual["candidate_burden"]
        for key in ("aggregate", "per_class", "candidate_burden", "timing"):
            row[key] = actual[key]
        diagnostics = operating_point(actual, expected[row["name"]]["confidence_threshold"])
        for key in ("geometric_error_categories", "failure_examples"):
            row[key] = diagnostics[key]


def render_failures(report, point, image_root, output):
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    from matplotlib.patches import Rectangle
    from PIL import Image
    examples = point["failure_examples"][:6]
    cases = {case["image"]: case for case in report["per_image"]}
    figure, axes = plt.subplots(2, 3, figsize=(18, 10), layout="constrained")
    for axis, example in zip(axes.flat, examples):
        case = cases[example["image"]]
        with Image.open(image_root / case["image"]) as image:
            axis.imshow(image)
        for label in case["truth"]:
            x1, y1, x2, y2 = label["bbox"]
            axis.add_patch(Rectangle((x1, y1), x2-x1, y2-y1, fill=False, color="#38c276", linewidth=.8, alpha=.4))
        if "prediction" in example:
            item = example["prediction"]
            x1, y1, x2, y2 = coordinates(item["bbox"])
            axis.add_patch(Rectangle((x1, y1), x2-x1, y2-y1, fill=False, color="#fc4646", linewidth=2.5))
            axis.text(x1, y2, f"{item['class_code']} {item['confidence']:.4f}", fontsize=7, color="white",
                      bbox={"facecolor": "#801515", "alpha": .85, "pad": 2})
            if case["truth"]:
                target = max(case["truth"], key=lambda label: overlap(coordinates(item["bbox"]), label["bbox"]))
                if overlap(coordinates(item["bbox"]), target["bbox"]) > 0:
                    gx1, gy1, gx2, gy2 = target["bbox"]
                    axis.add_patch(Rectangle((gx1, gy1), gx2-gx1, gy2-gy1, fill=False, color="#38c276", linewidth=2))
                    axis.text(gx1, gy1, target["class_code"], fontsize=8, color="white", bbox={"facecolor": "#153a22", "alpha": .85, "pad": 2})
        else:
            x1, y1, x2, y2 = case["truth"][example["truth_index"]]["bbox"]
            axis.add_patch(Rectangle((x1, y1), x2-x1, y2-y1, fill=False, color="#ffd33d", linewidth=3, linestyle="--"))
            axis.text(x1, y1, case["truth"][example["truth_index"]]["class_code"], fontsize=8,
                      color="#ffd33d", bbox={"facecolor": "#202020", "alpha": .85, "pad": 2})
        axis.set_title(example["kind"] + "\n" + Path(example["image"]).stem[:16], fontsize=10)
        axis.axis("off")
    for axis in list(axes.flat)[len(examples):]:
        axis.axis("off")
    figure.suptitle("Green: source annotation; red: unmatched prediction; yellow: missed annotation\n"
                    "First example of each geometric error type in image order; requires visual review.", fontsize=14)
    output.parent.mkdir(parents=True, exist_ok=True)
    figure.savefig(output, dpi=130)
    plt.close(figure)


def main():
    parser = argparse.ArgumentParser(description="Compare two validation factors on cached service rankings.")
    parser.add_argument("--primary", type=Path, required=True)
    parser.add_argument("--combined", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--candidate-config", type=Path)
    parser.add_argument("--frozen-selection", type=Path)
    parser.add_argument("--failure-figure", type=Path)
    parser.add_argument("--image-root", type=Path)
    parser.add_argument("--primary-point", type=Path)
    parser.add_argument("--baseline-point", type=Path)
    parser.add_argument("--candidate-point", type=Path)
    parser.add_argument("--freeze-output", type=Path)
    args = parser.parse_args()
    if bool(args.candidate_config) == bool(args.frozen_selection):
        parser.error("provide candidate-config for validation selection or frozen-selection for test reporting")
    if args.freeze_output and args.frozen_selection:
        parser.error("test reporting cannot create or change a selection freeze")
    if bool(args.failure_figure) != bool(args.image_root):
        parser.error("failure-figure and image-root must be provided together")
    if args.output.exists() or (args.candidate_config and args.candidate_config.exists()) or (args.freeze_output and args.freeze_output.exists()):
        parser.error("output already exists")
    primary, combined = (json.loads(path.read_text(encoding="utf-8")) for path in (args.primary, args.combined))
    if args.frozen_selection:
        frozen = json.loads(args.frozen_selection.read_text(encoding="utf-8"))
        result = summarize_frozen_test(primary, combined, frozen)
        if not all((args.primary_point, args.baseline_point, args.candidate_point)):
            parser.error("test reporting requires actual primary, baseline and candidate operating-point reports")
        point_paths = {"primary_control": args.primary_point, "current_deployment": args.baseline_point, "frozen_candidate": args.candidate_point}
        point_reports = {name: json.loads(path.read_text(encoding="utf-8")) for name, path in point_paths.items()}
        attach_operating_results(result, primary, combined, frozen, point_reports)
    else:
        result = compare(primary, combined)
    inputs = [args.primary, args.combined] + ([args.frozen_selection] if args.frozen_selection else [])
    if args.frozen_selection:
        inputs.extend(point_paths.values())
    result["input_sha256"] = {str(path.resolve()): file_hash(path) for path in inputs}
    result["analysis_source_sha256"] = file_hash(Path(__file__))
    if args.failure_figure:
        if args.failure_figure.exists():
            parser.error("failure figure already exists")
        baseline = next(row for row in result["experiments"] if row["name"].startswith("current_"))
        render_failures(combined, baseline, args.image_root, args.failure_figure)
    outputs = [(args.output, result)]
    if args.candidate_config:
        outputs.append((args.candidate_config, {"recognition": result["candidate_configuration"]}))
    for path, payload in outputs:
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("x", encoding="utf-8") as stream:
            json.dump(payload, stream, ensure_ascii=False, indent=2, allow_nan=False)
    if args.freeze_output:
        frozen = {"selected": result["selected"], "selection_dataset_sha256": result["dataset_sha256"],
                  "selection_report_sha256": file_hash(args.output), "candidate_config_sha256": file_hash(args.candidate_config),
                  "candidate_configuration": result["candidate_configuration"], "weights": result["weights"],
                  "baseline_confidence": .35, "selection_rule": result["selection_rule"],
                  "test_policy": "Evaluate held-out test after freezing; do not use test results for parameter selection. Prior test use prevents a claim of untouched independence."}
        args.freeze_output.parent.mkdir(parents=True, exist_ok=True)
        with args.freeze_output.open("x", encoding="utf-8") as stream:
            json.dump(frozen, stream, ensure_ascii=False, indent=2, allow_nan=False)
    print(result["selected"])
    for row in result["experiments"]:
        print(row["name"], row["aggregate"], row["candidate_burden"]["total"], row["ap"]["map50"], row["ap"]["map50_95"])


if __name__ == "__main__":
    main()
