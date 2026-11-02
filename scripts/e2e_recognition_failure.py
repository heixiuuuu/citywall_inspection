"""识别服务停止后，核验失败尝试持久化及人工补录、提交和审核。"""
import io
import json
import os
from pathlib import Path

from PIL import Image

from e2e_evidence import call, login


def main():
    inspector = login("inspector", "Insp123")
    reviewer = login("reviewer", "Review123")
    observation = call("POST", "/observations", inspector, 201,
                       json={"source": "VOLUNTARY", "location_text": "识别失败人工补录验收"})["observation"]
    oid = observation["id"]
    buffer = io.BytesIO()
    Image.new("RGB", (80, 60), "gray").save(buffer, "PNG")
    image = call("POST", f"/observations/{oid}/images", inspector, 201,
                 files={"image": ("sample.png", buffer.getvalue(), "image/png")},
                 data={"image_role": "OVERVIEW"})["image"]
    result = call("POST", f'/images/{image["id"]}/recognitions', inspector, json={})
    attempt = result["attempt"]
    assert attempt["status"] == "FAILED", attempt
    assert attempt["error_code"] == "RECOGNITION_FAILED", attempt
    assert result["detections"] == [], result
    call("PATCH", f"/observations/{oid}", inspector,
         json={"revision": 1, "latitude": 0, "longitude": 0, "accuracy_m": 5})
    call("PUT", f"/observations/{oid}/conclusion", inspector, 201,
         json={"revision": 2, "outcome": "SUSPECTED", "description": "软件验收人工补录，非现场鉴定",
               "items": [{"image_id": image["id"], "decision": "ADD", "final_class_code": "CRACK",
                          "description": "识别不可用时人工填写"}]})
    call("POST", f"/observations/{oid}/submit", inspector, json={"revision": 3})
    call("POST", f"/observations/{oid}/review", reviewer,
         json={"revision": 4, "decision": "ACCEPT", "disposition": "人工补录链路验收"})
    evidence = call("GET", f"/observations/{oid}", inspector)["evidence"]
    assert evidence["observation"]["status"] == "ACCEPTED"
    assert len(evidence["attempts"]) == 1
    saved_attempt = evidence["attempts"][0]
    for field in ("id", "status", "error_code"):
        assert saved_attempt[field] == attempt[field], saved_attempt
    assert saved_attempt["finished_at"]
    assert evidence["items"][0]["decision"] == "ADD"
    assert not evidence["items"][0].get("source_detection_id")
    assert len(evidence["submissions"]) == 1
    if os.getenv("CITY_WALL_TEST_PDF"):
        Path(os.environ["CITY_WALL_TEST_PDF"]).with_name("recognition-failure.json").write_text(
            json.dumps(evidence, ensure_ascii=False, indent=2), encoding="utf-8")
    print("Recognition unavailable: failed attempt persisted; manual conclusion submitted and accepted")


if __name__ == "__main__":
    main()
