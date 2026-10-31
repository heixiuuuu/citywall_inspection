"""使用真实图片核验识别、人工复核与报告链路；仅在隔离演示环境执行。"""
import argparse
import json
import os
from pathlib import Path

import requests


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--image", type=Path, required=True)
    parser.add_argument("--require-vision", action="store_true")
    parser.add_argument("--confirm-class", help="确认指定类别的一个识别候选")
    parser.add_argument("--output", type=Path, default=Path("C:/code/tmp/citywall-model-e2e"))
    args = parser.parse_args()
    base = os.getenv("CITY_WALL_API_BASE", "http://127.0.0.1:8080/api/v1")

    def call(method, path, headers=None, **kwargs):
        response = requests.request(method, base + path, headers=headers, timeout=110, **kwargs)
        response.raise_for_status()
        return response.json()["data"]

    admin = call("POST", "/auth/login", json={"username": "admin", "password": "Admin123"})
    inspector = call("POST", "/auth/login", json={"username": "inspector", "password": "Insp123"})
    admin_headers = {"Authorization": "Bearer " + admin["token"]}
    inspector_headers = {"Authorization": "Bearer " + inspector["token"]}
    point = call("GET", "/inspection-points", admin_headers)["items"][0]
    task = call("POST", "/tasks", admin_headers, json={
        "title": "公开数据集模型联调演示", "assignee_id": inspector["user"]["id"],
        "point_ids": [point["id"]], "description": "仅验证软件链路，样本不作为点位现场照片或病害鉴定依据。",
    })["task"]
    tid = task["id"]
    call("POST", f"/tasks/{tid}/dispatch", admin_headers, json={"revision": task["revision"]})
    task_point = call("GET", f"/tasks/{tid}", admin_headers)["points"][0]
    observation = call("POST", "/observations", inspector_headers, json={
        "source": "TASK", "task_point_id": task_point["id"], "location_text": "公开数据集联调样本，非现场巡检",
    })["observation"]
    oid = observation["id"]
    with args.image.open("rb") as source:
        image = call("POST", f"/observations/{oid}/images", inspector_headers,
                     files={"image": (args.image.name, source, "image/jpeg")},
                     data={"image_role": "OVERVIEW"})["image"]
    result = call("POST", f'/images/{image["id"]}/recognitions', inspector_headers, json={})
    assert result["attempt"]["status"] == "SUCCEEDED", result["attempt"].get("error_code")
    analysis = json.loads(result["attempt"].get("analysis_json") or "null")
    if args.require_vision:
        assert analysis and analysis["status"] == "SUCCEEDED", analysis
        assert analysis["description"].strip()
    for detection in result["detections"]:
        assert 0 <= detection["x1"] < detection["x2"] <= image["width"]
        assert 0 <= detection["y1"] < detection["y2"] <= image["height"]
        assert 0 <= detection["confidence"] <= 1
    items = []
    if args.confirm_class:
        candidate = next((d for d in result["detections"] if d["class_code"] == args.confirm_class), None)
        assert candidate, f"missing candidate: {args.confirm_class}"
        items.append({"image_id": image["id"], "source_detection_id": candidate["id"], "decision": "CONFIRM",
                      "final_class_code": args.confirm_class, "description": "仅验证候选复核流程。"})
    call("PUT", f"/observations/{oid}/conclusion", inspector_headers, json={
        "revision": observation["revision"], "outcome": "SUSPECTED" if items else "UNDETERMINED", "items": items,
        "description": "公开数据集联调样本，人工未作现场判断。",
    })
    call("POST", f"/observations/{oid}/submit", inspector_headers, json={"revision": 2})
    current_task = call("GET", f"/tasks/{tid}", inspector_headers)["task"]
    call("POST", f"/tasks/{tid}/submit", inspector_headers, json={"revision": current_task["revision"]})
    call("POST", f"/observations/{oid}/review", admin_headers, json={
        "revision": 3, "decision": "ACCEPT", "disposition": "仅确认演示数据流转完整，不代表病害鉴定。",
    })
    current_task = call("GET", f"/tasks/{tid}", admin_headers)["task"]
    call("POST", f"/tasks/{tid}/complete", admin_headers, json={"revision": current_task["revision"]})
    report = call("POST", f"/reports/task/{tid}", admin_headers)["report"]
    evidence = call("GET", f"/observations/{oid}", admin_headers)["evidence"]
    if items:
        assert any(item["source_detection_id"] == candidate["id"] and item["final_class_code"] == args.confirm_class
                   for item in evidence["items"])
    assert json.loads(evidence["attempts"][0]["analysis_json"]) == analysis
    assert evidence["observation"]["status"] == "ACCEPTED"
    response = requests.get(base + f'/reports/{report["id"]}/file', headers=admin_headers, timeout=30)
    response.raise_for_status()
    assert response.content.startswith(b"%PDF-")
    args.output.mkdir(parents=True, exist_ok=True)
    (args.output / "report.pdf").write_bytes(response.content)
    (args.output / "evidence.json").write_text(json.dumps(evidence, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"task_id": tid, "observation_id": oid, "detections": len(result["detections"]),
                      "vision_status": analysis["status"] if analysis else "DISABLED", "output": str(args.output)}))


if __name__ == "__main__":
    main()
