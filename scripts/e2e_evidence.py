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
    call("PUT", f'/inspection-points/{point["id"]}', admin, json={**point_payload, "active": False})
    call("POST", "/tasks", admin, 400, json={"title": "停用点位", "assignee_id": 3, "point_ids": [point["id"]]})
    call("PUT", f'/inspection-points/{point["id"]}', admin, json={**point_payload, "name": "已维护点位"})
    other_inspector = login("inspector_other", "Insp123")
    other_id = call("GET", "/me", other_inspector)["user"]["id"]
    denied_task = call("POST", "/tasks", admin, 201, json={"title": "权限验收", "assignee_id": other_id, "point_ids": [point["id"]]})["task"]
    call("POST", f'/tasks/{denied_task["id"]}/dispatch', admin, json={"revision": denied_task["revision"]})
    task_point = call("GET", f'/tasks/{denied_task["id"]}', admin)["points"][0]
    call("POST", "/observations", inspector, 403, json={"source": "TASK", "task_point_id": task_point["id"], "location_text": "其他人的任务"})
    observation = call("POST", "/observations", inspector, 201, json={"source": "VOLUNTARY", "location_text": "证据验收"})["observation"]
    oid = observation["id"]
    buffer = io.BytesIO()
    Image.new("RGB", (80, 60), "gray").save(buffer, "PNG")
    image = call("POST", f"/observations/{oid}/images", inspector, 201, files={"image": ("sample.png", buffer.getvalue(), "image/png")}, data={"image_role": "OVERVIEW"})["image"]
    iid = image["id"]
    exif = Image.Exif()
    exif[274] = 6
    rotated = io.BytesIO()
    Image.new("RGB", (80, 60), "gray").save(rotated, "JPEG", exif=exif)
    raw = rotated.getvalue()
    oriented = call("POST", f"/observations/{oid}/images", inspector, 201,
                    files={"image": ("oriented.jpg", raw, "image/jpeg")}, data={"image_role": "DETAIL"})["image"]
    original_response = requests.get(BASE + f'/images/{oriented["id"]}/content', headers=inspector, timeout=10)
    assert original_response.status_code == 200 and original_response.content == raw
    preview_response = requests.get(BASE + f'/images/{oriented["id"]}/preview', headers=inspector, timeout=10)
    assert preview_response.status_code == 200
    preview_image = Image.open(io.BytesIO(preview_response.content))
    assert preview_image.size == (60, 80) and not preview_image.getexif()
    for endpoint in ("content", "preview"):
        response = requests.get(BASE + f'/images/{oriented["id"]}/{endpoint}', headers=other_inspector, timeout=10)
        assert response.status_code == 403
    call("POST", f"/observations/{oid}/images", inspector, 422,
         files={"image": ("invalid.png", b"not an image", "image/png")})
    call("POST", f"/observations/{oid}/images", inspector, 422,
         files={"image": ("mismatch.png", raw, "image/png")})
    attempt = call("POST", f"/images/{iid}/recognitions", inspector, json={})["attempt"]
    assert attempt["status"] in ("SUCCEEDED", "FAILED")
    evidence = call("GET", f"/observations/{oid}", inspector)["evidence"]
    assert evidence["attempts"][0]["id"] == attempt["id"]
    call("PATCH", f"/observations/{oid}", inspector, json={"revision": 1, "latitude": 0, "longitude": 0, "accuracy_m": 5, "inspection_outcome": "NORMAL"})
    call("PUT", f"/observations/{oid}/conclusion", inspector, 201, json={"revision": 2, "outcome": "NORMAL", "description": "现场人工复核", "items": []})
    before_invalid_box = call("GET", f"/observations/{oid}", inspector)["evidence"]
    call("PUT", f"/observations/{oid}/conclusion", inspector, 400, json={"revision": 3, "outcome": "SUSPECTED", "description": "无效坐标不得覆盖人工结论",
         "items": [{"image_id": oriented["id"], "decision": "ADD", "final_class_code": "CRACK", "final_box_json": '{"x1":0,"y1":0,"x2":70,"y2":20}'}]})
    after_invalid_box = call("GET", f"/observations/{oid}", inspector)["evidence"]
    assert after_invalid_box["observation"]["revision"] == 3
    assert after_invalid_box["conclusions"] == before_invalid_box["conclusions"] and after_invalid_box["items"] == before_invalid_box["items"]
    valid_oriented_box = '{"x1":0,"y1":0,"x2":60,"y2":80}'
    call("PUT", f"/observations/{oid}/conclusion", inspector, 201, json={"revision": 3, "outcome": "UNDETERMINED", "description": "仅验证旋转照片坐标，非现场结论",
         "items": [{"image_id": oriented["id"], "decision": "ADD", "final_class_code": "CRACK", "final_box_json": valid_oriented_box}]})
    oriented_evidence = call("GET", f"/observations/{oid}", inspector)["evidence"]
    assert oriented_evidence["items"][-1]["final_box_json"] == valid_oriented_box
    call("POST", f"/observations/{oid}/submit", inspector, json={"revision": 4})
    call("POST", f"/observations/{oid}/review", admin, json={"revision": 5, "decision": "ACCEPT", "disposition": "证据完整"})
    evidence = call("GET", f"/observations/{oid}", inspector)["evidence"]
    assert evidence["observation"]["submitted_at"] and evidence["observation"]["reviewed_at"]
    assert evidence["conclusions"][0]["description"] == "现场人工复核"
    assert evidence["reviews"][0]["decision"] == "ACCEPT"
    call("DELETE", f"/observations/{oid}/images/{iid}", inspector, 409)
    call("POST", f"/images/{iid}/recognitions", inspector, 409, json={})
    assert len(call("GET", f"/observations/{oid}", inspector)["evidence"]["attempts"]) == 1
    call("GET", f"/observations/{oid}", other_inspector, 403)
    issue_id = call("POST", f"/observations/{oid}/admit", admin,
                    json={"new_issues": [{"title": "责任人证据权限验收", "priority_code": "WATCH"}]})["admit"]["created_issue_ids"][0]
    issue = call("POST", f"/issues/{issue_id}/assign", admin,
                 json={"revision": 1, "assignee_id": other_id})["issue"]
    linked = call("GET", f"/observations/{oid}", other_inspector)["evidence"]
    assert linked["observation"]["id"] == oid and linked["reviews"][0]["decision"] == "ACCEPT"
    assert any(meta["id"] == iid for meta in call("GET", f"/observations/{oid}/images", other_inspector)["images"])
    read_original = requests.get(BASE + f"/images/{iid}/content", headers=other_inspector, timeout=10)
    assert read_original.status_code == 200 and read_original.content == buffer.getvalue()
    read_preview = requests.get(BASE + f'/images/{oriented["id"]}/preview', headers=other_inspector, timeout=10)
    assert read_preview.status_code == 200 and Image.open(io.BytesIO(read_preview.content)).size == (60, 80)
    call("PATCH", f"/observations/{oid}", other_inspector, 403, json={"revision": linked["observation"]["revision"], "location_text": "越权修改"})
    call("POST", f"/images/{iid}/recognitions", other_inspector, 403, json={})
    own_records = call("GET", "/observations?page_size=100", other_inspector)["items"]
    assert all(record["id"] != oid for record in own_records)
    call("POST", f"/issues/{issue_id}/assign", admin, json={"revision": issue["revision"], "assignee_id": 3})
    call("GET", f"/observations/{oid}", other_inspector, 403)
    call("GET", f"/observations/{oid}/images", other_inspector, 403)
    assert requests.get(BASE + f"/images/{iid}/content", headers=other_inspector, timeout=10).status_code == 403
    assert requests.get(BASE + f'/images/{oriented["id"]}/preview', headers=other_inspector, timeout=10).status_code == 403
    draft = call("POST", "/observations", inspector, 201, json={"source": "VOLUNTARY", "location_text": "移除照片验收"})["observation"]
    removable = call("POST", f'/observations/{draft["id"]}/images', inspector, 201, files={"image": ("sample.png", buffer.getvalue(), "image/png")})["image"]
    call("DELETE", f'/observations/{draft["id"]}/images/{removable["id"]}', inspector)
    assert call("GET", f'/observations/{draft["id"]}/images', inspector)["images"] == []
    did = draft["id"]
    historical_image = call("POST", f"/observations/{did}/images", inspector, 201, files={"image": ("sample.png", buffer.getvalue(), "image/png")}, data={"image_role": "OVERVIEW"})["image"]
    historical_attempt = call("POST", f'/images/{historical_image["id"]}/recognitions', inspector, json={})["attempt"]
    call("PATCH", f"/observations/{did}", inspector, json={"revision": 1, "latitude": 0, "longitude": 0, "location_text": "退回补正位置"})
    call("PUT", f"/observations/{did}/conclusion", inspector, 201, json={"revision": 2, "outcome": "SUSPECTED", "description": "保留待补正描述", "items": [{"image_id": historical_image["id"], "decision": "ADD", "final_class_code": "CRACK", "final_box_json": '{"x1":1,"y1":1,"x2":20,"y2":20}', "description": "软件追溯验证"}]})
    call("POST", f"/observations/{did}/submit", inspector, json={"revision": 3})
    call("POST", f"/observations/{did}/review", admin, json={"revision": 4, "decision": "RETURN", "reason": "请补充墙体近景"})
    returned = call("GET", f"/observations/{did}", inspector)["evidence"]
    assert returned["observation"]["status"] == "RETURNED" and returned["reviews"][-1]["reason"] == "请补充墙体近景"
    assert returned["conclusions"][-1]["description"] == "保留待补正描述"
    first_submission = returned["submissions"][0]
    assert first_submission["location"]["location_text"] == "退回补正位置"
    call("DELETE", f'/observations/{did}/images/{historical_image["id"]}', inspector)
    assert requests.get(BASE + f'/images/{historical_image["id"]}/content', headers=inspector, timeout=10).content == buffer.getvalue()
    assert requests.get(BASE + f'/images/{removable["id"]}/content', headers=inspector, timeout=10).status_code == 403
    assert requests.get(BASE + f'/images/{historical_image["id"]}/preview', headers=other_inspector, timeout=10).status_code == 403
    replacement_bytes = io.BytesIO()
    Image.new("RGB", (80, 60), "steelblue").save(replacement_bytes, "PNG")
    replacement = call("POST", f"/observations/{did}/images", inspector, 201, files={"image": ("replacement.png", replacement_bytes.getvalue(), "image/png")}, data={"image_role": "OVERVIEW"})["image"]
    call("POST", f"/observations/{did}/submit", inspector, 400, json={"revision": 5})
    unchanged = call("GET", f"/observations/{did}", inspector)["evidence"]
    assert len(unchanged["submissions"]) == 1 and unchanged["observation"]["revision"] == 5
    assert any(attempt["id"] == historical_attempt["id"] for attempt in unchanged["attempts"])
    assert [image["id"] for image in call("GET", f"/observations/{did}/images", inspector)["images"]] == [replacement["id"]]
    call("PATCH", f"/observations/{did}", inspector, json={"revision": 5, "latitude": 1, "longitude": 1, "location_text": "补正后位置"})
    call("PUT", f"/observations/{did}/conclusion", inspector, 201, json={"revision": 6, "outcome": "UNDETERMINED", "description": "补正后待复核", "items": []})
    call("POST", f"/observations/{did}/submit", inspector, json={"revision": 7})
    corrected = call("GET", f"/observations/{did}", inspector)["evidence"]
    assert len(corrected["submissions"]) == 2
    old, new = corrected["submissions"]
    assert old["location"]["latitude"] == 0 and new["location"]["latitude"] == 1
    assert old["images"][0]["id"] == historical_image["id"] and old["images"][0]["role"] == "ARCHIVED"
    assert new["images"][0]["id"] == replacement["id"] and old["conclusion_revision_id"] != new["conclusion_revision_id"]
    call("POST", f"/observations/{did}/review", admin, json={"revision": 8, "decision": "ACCEPT", "disposition": "软件追溯验证"})
    archived_issue = call("POST", f"/observations/{did}/admit", admin, json={"new_issues": [{"title": "历史提交读取权限验收", "priority_code": "WATCH"}]})["admit"]["created_issue_ids"][0]
    assigned = call("POST", f"/issues/{archived_issue}/assign", admin, json={"revision": 1, "assignee_id": other_id})["issue"]
    assert requests.get(BASE + f'/images/{historical_image["id"]}/preview', headers=other_inspector, timeout=10).status_code == 200
    call("POST", f"/issues/{archived_issue}/assign", admin, json={"revision": assigned["revision"], "assignee_id": 3})
    assert requests.get(BASE + f'/images/{historical_image["id"]}/preview', headers=other_inspector, timeout=10).status_code == 403
    private = call("POST", "/observations", admin, 201, json={"source": "VOLUNTARY", "location_text": "其他人的记录"})["observation"]
    call("GET", f'/observations/{private["id"]}', inspector, 403)
    call("GET", f'/observations/{private["id"]}', other_inspector, 403)
    own_task = call("POST", "/tasks", admin, 201, json={"title": "记录定位查询", "assignee_id": 3, "point_ids": [point["id"]]})["task"]
    assignees = call("GET", "/tasks/assignees", admin)["items"]
    assert any(user["id"] == 3 for user in assignees) and all(user["id"] != 1 for user in assignees)
    call("GET", "/tasks/assignees", inspector, 403)
    call("POST", "/tasks", admin, 400, json={"title": "无效负责人", "assignee_id": 1, "point_ids": [point["id"]]})
    call("POST", f'/tasks/{own_task["id"]}/dispatch', admin, json={"revision": 1})
    own_point = call("GET", f'/tasks/{own_task["id"]}', inspector)["points"][0]
    own_record = call("POST", "/observations", inspector, 201, json={"source": "TASK", "task_point_id": own_point["id"], "location_text": "任务草稿"})["observation"]
    filtered = call("GET", f'/observations?task_point_id={own_point["id"]}&page_size=1', inspector)
    assert filtered["total"] == 1 and filtered["items"][0]["id"] == own_record["id"]
    call("POST", "/auth/wechat-login", {}, 401, json={"code": "test-admin"})
    location_payload = {**point_payload, "code": code + "-location", "name": "定位软件验收", "radius_m": 100}
    location_point = call("POST", "/inspection-points", admin, 201, json=location_payload)["point"]
    location_task = call("POST", "/tasks", admin, 201, json={"title": "定位软件验收（非现场巡检）", "assignee_id": 3, "point_ids": [location_point["id"]]})["task"]
    tid = location_task["id"]
    call("POST", f"/tasks/{tid}/dispatch", admin, json={"revision": 1})
    frozen_point = call("GET", f"/tasks/{tid}", inspector)["points"][0]
    call("PUT", f'/inspection-points/{location_point["id"]}', admin,
         json={**location_payload, "latitude": 10, "longitude": 10, "radius_m": 300})
    location_record = call("POST", "/observations", inspector, 201,
                           json={"source": "TASK", "task_point_id": frozen_point["id"], "location_text": "软件定位验收（非现场巡检）"})["observation"]
    lid = location_record["id"]
    call("POST", f"/observations/{lid}/images", inspector, 201,
         files={"image": ("location.png", buffer.getvalue(), "image/png")}, data={"image_role": "OVERVIEW"})
    for invalid in ({"latitude": 91, "longitude": 0}, {"latitude": 0}, {"accuracy_m": -1}):
        call("PATCH", f"/observations/{lid}", inspector, 400,
             json={"revision": 1, "location_text": "不得保存", **invalid})
    unchanged = call("GET", f"/observations/{lid}", inspector)["evidence"]["observation"]
    assert unchanged["revision"] == 1 and unchanged["latitude"] is None
    assert unchanged["location_text"] == location_record["location_text"]
    call("PATCH", f"/observations/{lid}", inspector, json={"revision": 1, "latitude": 0, "longitude": 0, "accuracy_m": 0})
    origin = call("GET", f"/observations/{lid}", inspector)["evidence"]["observation"]
    assert origin["distance_m"] == 0 and origin["accuracy_m"] == 0
    call("PATCH", f"/observations/{lid}", inspector, json={"revision": 2, "latitude": .001, "longitude": 0, "accuracy_m": 45})
    call("PUT", f"/observations/{lid}/conclusion", inspector, 201,
         json={"revision": 3, "outcome": "UNDETERMINED", "description": "软件定位验收，待现场复核", "items": []})
    call("POST", f"/observations/{lid}/submit", inspector, json={"revision": 4})
    submitted = call("GET", f"/observations/{lid}", inspector)["evidence"]
    assert submitted["location_radius_m"] == 100 and submitted["submissions"][0]["location"]["radius_m"] == 100
    assert abs(submitted["submissions"][0]["location"]["distance_m"] - 111.19) < .02
    call("POST", f"/observations/{lid}/review", admin,
         json={"revision": 5, "decision": "RETURN", "disposition": "软件验收位置补正", "reason": "补充现场位置"})
    call("PATCH", f"/observations/{lid}", inspector, json={"revision": 6, "latitude": .002})
    corrected_location = call("GET", f"/observations/{lid}", inspector)["evidence"]
    assert abs(corrected_location["observation"]["distance_m"] - 222.39) < .02
    assert corrected_location["location_radius_m"] == 100
    assert corrected_location["submissions"] == submitted["submissions"]
    location_report = call("POST", f"/reports/task/{tid}", admin, 201)["report"]
    pdf = requests.get(BASE + f'/reports/{location_report["id"]}/file', headers=admin, timeout=30)
    assert pdf.status_code == 200
    import pymupdf
    with pymupdf.open(stream=pdf.content, filetype="pdf") as document:
        report_text = "".join(page.get_text() for page in document)
    for expected in ("超出任务点提示半径", "定位精度超过 30m", "提示半径：100.0 m", "距任务目标：222.4 m"):
        assert expected in report_text, (expected, report_text)
    if os.getenv("CITY_WALL_TEST_PDF"):
        from pathlib import Path
        Path(os.environ["CITY_WALL_TEST_PDF"]).with_name("location-report.pdf").write_bytes(pdf.content)
    print("Evidence, location snapshots, returned drafts, authorization and persisted timestamps passed")


if __name__ == "__main__":
    main()
