import copy
import json
import zipfile

from PIL import Image
import pytest
import yaml

from scripts.audit_model_data import audit
from scripts.compare_model_experiments import attach_operating_results, average_precision, compare, operating_point, reroute, summarize_frozen_test


def ranking():
    return {"split": "val", "dataset_sha256": "same", "weights": [{"sha256": "primary"}], "match_iou": .5,
            "configuration": {"specialist_class_codes": ["CRACK", "EFFLORESCENCE", "MOSS"]},
            "annotation_coverage": {"classes": ["CRACK", "PLANT"]},
            "runtime_members": [{"classes": {0: "CRACK", 1: "PLANT"}, "parameters": {"conf": .001}}],
            "per_image": [{"image": "val/image.jpg", "image_sha256": "same", "width": 100, "height": 100,
                           "truth": [{"class_code": "CRACK", "bbox": [0, 0, 10, 10]}],
                           "predictions": [{"detection_id": "d0", "class_code": "CRACK", "confidence": .9,
                                            "bbox": {"x1": 0, "y1": 0, "x2": 10, "y2": 10}}]}]}


def test_ap_uses_low_score_rankings_and_penalizes_high_score_false_positive():
    pytest.importorskip("ultralytics")
    report = ranking()
    perfect = average_precision(report)
    assert perfect["map50"] > .99 and perfect["map50_95"] > .99
    assert perfect["per_class"]["PLANT"]["map50"] is None
    report["per_image"][0]["predictions"][0]["bbox"] = {"y2": 10, "x2": 10, "y1": 0, "x1": 0}
    assert average_precision(report)["map50"] == perfect["map50"]
    wrong = copy.deepcopy(report["per_image"][0]["predictions"][0])
    wrong.update(confidence=.95, bbox={"x1": 20, "y1": 20, "x2": 30, "y2": 30})
    report["per_image"][0]["predictions"].append(wrong)
    assert .49 < average_precision(report)["map50"] < .51
    report["runtime_members"][0]["parameters"]["conf"] = .35
    with pytest.raises(ValueError, match="AP requires"):
        average_precision(report)


def test_selection_rejects_test_split_before_scoring():
    report = ranking()
    report["split"] = "test"
    with pytest.raises(ValueError, match="validation reports only"):
        compare(report, ranking())


def test_reroute_restores_primary_class_without_duplicate_candidates():
    primary, combined = ranking(), ranking()
    item = copy.deepcopy(primary["per_image"][0]["predictions"][0])
    item["class_code"] = "EFFLORESCENCE"
    primary["per_image"][0]["predictions"].append(item)
    bad = copy.deepcopy(item)
    bad["confidence"] = .3
    combined["per_image"][0]["predictions"].append(bad)
    result = reroute(primary, combined, ["CRACK", "MOSS"])
    restored = [item for item in result["per_image"][0]["predictions"] if item["class_code"] == "EFFLORESCENCE"]
    assert len(restored) == 1 and restored[0]["confidence"] == .9
    assert combined["per_image"][0]["predictions"][-1]["confidence"] == .3
    primary["dataset_sha256"] = "different"
    with pytest.raises(ValueError, match="identical dataset"):
        reroute(primary, combined, ["CRACK"])


def test_reroute_rejects_changed_primary_inference_parameters():
    primary, combined = ranking(), ranking()
    primary["runtime_members"][0]["parameters"]["conf"] = .002
    with pytest.raises(ValueError, match="identical primary inference"):
        reroute(primary, combined, ["CRACK"])


def test_frozen_test_summary_rejects_changed_weights_before_scoring():
    primary, combined = ranking(), ranking()
    primary["split"] = combined["split"] = "test"
    with pytest.raises(ValueError, match="test weights differ"):
        summarize_frozen_test(primary, combined, {"weights": []})


def test_actual_operating_results_take_precedence_over_rounded_cache_filter():
    primary, combined = ranking(), ranking()
    config = {**combined["configuration"], "confidence_threshold": .2}
    frozen = {"candidate_configuration": config, "baseline_confidence": .35, "weights": combined["weights"]}
    summary = {"dataset_sha256": "same", "experiments": [{"name": "frozen_candidate", "aggregate": {"tp": 1}, "candidate_burden": {"total": 1}}]}
    actual = {**ranking(), "split": "test", "dataset_sha256": "same", "configuration": config, "weights": combined["weights"],
              "aggregate": {"tp": 0, "fp": 0, "fn": 1}, "candidate_burden": {"total": 0}, "per_class": {}, "timing": {"scope": "actual"}}
    actual["per_image"][0]["predictions"] = []
    attach_operating_results(summary, primary, combined, frozen, {"frozen_candidate": actual})
    assert summary["experiments"][0]["aggregate"]["fn"] == 1
    assert summary["experiments"][0]["cache_metrics_match_actual"] is False
    assert summary["experiments"][0]["geometric_error_categories"]["fn_no_overlapping_candidate"] == 1


def test_threshold_burden_keeps_unassessed_spalling_candidates():
    report = ranking()
    item = copy.deepcopy(report["per_image"][0]["predictions"][0])
    item.update(class_code="SURFACE_SPALLING", confidence=.25)
    report["per_image"][0]["predictions"].append(item)
    result = operating_point(report, .35)
    assert result["aggregate"]["tp"] == 1 and result["aggregate"]["fp"] == 0
    assert result["candidate_burden"]["total"] == 2
    assert result["candidate_burden"]["unassessed_by_class"] == {"SURFACE_SPALLING": 1}


def test_data_audit_detects_duplicate_splits_and_checks_voc(tmp_path):
    records = []
    archive_path = tmp_path / "source.zip"
    with zipfile.ZipFile(archive_path, "w") as archive:
        for split in ("train", "val"):
            (tmp_path / "images" / split).mkdir(parents=True)
            (tmp_path / "labels" / split).mkdir(parents=True)
            Image.new("RGB", (20, 10), "gray").save(tmp_path / "images" / split / f"{split}.jpg")
            (tmp_path / "labels" / split / f"{split}.txt").write_text("0 .5 .5 .5 .6" if split == "train" else "")
            obj = "<object><name>cracks</name><bndbox><xmin>5</xmin><ymin>2</ymin><xmax>15</xmax><ymax>8</ymax></bndbox></object>" if split == "train" else ""
            archive.writestr(f"{split}.xml", f"<annotation><size><width>20</width><height>10</height></size>{obj}</annotation>")
            records.append({"sha256": split, "split": split, "source": f"{split}.jpg", "annotation": f"{split}.xml"})
    (tmp_path / "manifest.json").write_text(json.dumps({"images": records}))
    (tmp_path / "data.yaml").write_text(yaml.safe_dump({"names": {0: "CRACK"}}))
    result = audit(tmp_path, archive_path)
    assert result["label_issues"] == []
    assert result["negative_images"] == {"train": 0, "val": 1}
    assert result["exact_cross_split_duplicates"] == [["train/train.jpg", "val/val.jpg"]]
    assert len(result["near_cross_split_pairs"]) == 1
