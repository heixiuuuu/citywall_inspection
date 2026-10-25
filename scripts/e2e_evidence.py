"""核验档案维护、任务权限、现场证据和状态时间的真实持久化。"""
import io
import os
import requests
from PIL import Image

BASE = os.getenv("CITY_WALL_API_BASE", "http://127.0.0.1:8080/api/v1")


def login(username, password):
    response = requests.post(BASE + "/auth/login", json={"username": username, "password": password}, timeout=10)
    response.raise_for_status()
    return {"Authorization": "Bearer " + response.json()["data"]["token"]}


def call(method, path, headers, expected=200, **kwargs):
    response = requests.request(method, BASE + path, headers=headers, timeout=120, **kwargs)
    assert response.status_code == expected, (path, response.status_code, response.text)
    return response.json().get("data")


def main():
    admin, inspector = login("admin", "Admin123"), login("inspector", "Insp123")
    import uuid
    code = "verify-" + uuid.uuid4().hex[:10]
    segment = call("POST", "/wall-segments", admin, 201, json={"code": code, "name": "验收区段", "description": "巡检区域档案"})["segment"]
    point_payload = {"segment_id": segment["id"], "code": code, "name": "验收点位", "latitude": 0, "longitude": 0, "radius_m": 50, "active": True}
    point = call("POST", "/inspection-points", admin, 201, json=point_payload)["point"]
    call("PUT", f'/inspection-points/{point["id"]}', inspector, 403, json=point_payload)
    call("PUT", f'/inspection-points/{point["id"]}', admin, json={**point_payload, "name": "已维护点位"})
    denied_task = call("POST", "/tasks", admin, 201, json={"title": "权限验收", "assignee_id": 1, "point_ids": [point["id"]]})["task"]
    call("POST", f'/tasks/{denied_task["id"]}/dispatch', admin, json={"revision": denied_task["revision"]})
    task_point = call("GET", f'/tasks/{denied_task["id"]}', admin)["points"][0]
    call("POST", "/observations", inspector, 403, json={"source": "TASK", "task_point_id": task_point["id"], "location_text": "其他人的任务"})
    observation = call("POST", "/observations", inspector, 201, json={"source": "VOLUNTARY", "location_text": "证据验收"})["observation"]
    oid = observation["id"]
    buffer = io.BytesIO()
    Image.new("RGB", (80, 60), "gray").save(buffer, "PNG")
    image = call("POST", f"/observations/{oid}/images", inspector, 201, files={"image": ("sample.png", buffer.getvalue(), "image/png")}, data={"image_role": "OVERVIEW"})["image"]
    iid = image["id"]
    attempt = call("POST", f"/images/{iid}/recognitions", inspector, json={})["attempt"]
    assert attempt["status"] in ("SUCCEEDED", "FAILED")
    evidence = call("GET", f"/observations/{oid}", inspector)["evidence"]
    assert evidence["attempts"][0]["id"] == attempt["id"]
    call("PATCH", f"/observations/{oid}", inspector, json={"revision": 1, "latitude": 0, "longitude": 0, "accuracy_m": 5, "inspection_outcome": "NORMAL"})
    call("PUT", f"/observations/{oid}/conclusion", inspector, 201, json={"revision": 2, "outcome": "NORMAL", "description": "现场人工复核", "items": []})
    call("POST", f"/observations/{oid}/submit", inspector, json={"revision": 3})
    call("POST", f"/observations/{oid}/review", admin, json={"revision": 4, "decision": "ACCEPT", "disposition": "证据完整"})
    evidence = call("GET", f"/observations/{oid}", inspector)["evidence"]
    assert evidence["observation"]["submitted_at"] and evidence["observation"]["reviewed_at"]
    assert evidence["conclusions"][0]["description"] == "现场人工复核"
    assert evidence["reviews"][0]["decision"] == "ACCEPT"
    call("DELETE", f"/observations/{oid}/images/{iid}", inspector, 409)
    draft = call("POST", "/observations", inspector, 201, json={"source": "VOLUNTARY", "location_text": "移除照片验收"})["observation"]
    removable = call("POST", f'/observations/{draft["id"]}/images', inspector, 201, files={"image": ("sample.png", buffer.getvalue(), "image/png")})["image"]
    call("DELETE", f'/observations/{draft["id"]}/images/{removable["id"]}', inspector)
    assert call("GET", f'/observations/{draft["id"]}/images', inspector)["images"] == []
    call("POST", "/auth/wechat-login", {}, 401, json={"code": "test-admin"})
    print("Evidence, archive, assignment authorization, image removal and persisted timestamps passed")


if __name__ == "__main__":
    main()
