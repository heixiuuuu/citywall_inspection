"""验证报告照片、受理结果、处置历史及快照不变性。"""
import pymupdf
import requests
import os
import sys
import io
from datetime import datetime, timedelta, timezone
from PIL import Image

BASE = os.getenv("CITY_WALL_API_BASE", "http://127.0.0.1:8080/api/v1")
OUT = os.getenv("CITY_WALL_TEST_PDF", "C:/code/tmp/citywall-report-test.pdf")


def check(name, r, expect=None):
    ok = expect is None or r.status_code == expect
    detail = f"PDF {len(r.content)} bytes" if r.headers.get("Content-Type", "").startswith("application/pdf") else r.text[:140]
    print(f"{name:24} -> {r.status_code} {'OK ' if ok else 'FAIL'} {detail}")
    if not ok:
        sys.exit(1)
    return r


def main():
    t = requests.post(f"{BASE}/auth/login", json={"username": "admin", "password": "Admin123"})
    t.raise_for_status()
    H = {"Authorization": "Bearer " + t.json()["data"]["token"], "Content-Type": "application/json"}

    ti = requests.post(f"{BASE}/auth/login", json={"username": "inspector", "password": "Insp123"}, timeout=10).json()["data"]["token"]
    inspector = {"Authorization": "Bearer " + ti}
    point = requests.get(f"{BASE}/inspection-points", headers=H, timeout=10).json()["data"]["items"][0]
    task = requests.post(f"{BASE}/tasks", headers=H, json={"title": "报告照片与中文验收", "assignee_id": 3, "point_ids": [point["id"]]}, timeout=10).json()["data"]["task"]
    task_id = task["id"]
    requests.post(f"{BASE}/tasks/{task_id}/dispatch", headers=H, json={"revision": 1}, timeout=10).raise_for_status()
    task_point = requests.get(f"{BASE}/tasks/{task_id}", headers=H, timeout=10).json()["data"]["points"][0]
    oid = requests.post(f"{BASE}/observations", headers=inspector, json={"source": "TASK", "task_point_id": task_point["id"], "location_text": "城墙现场"}, timeout=10).json()["data"]["observation"]["id"]
    buffer = io.BytesIO()
    Image.new("RGB", (320, 240), (180, 120, 80)).save(buffer, "PNG")
    requests.post(f"{BASE}/observations/{oid}/images", headers=inspector, files={"image": ("现场.png", buffer.getvalue(), "image/png")}, timeout=10).raise_for_status()
    requests.put(f"{BASE}/observations/{oid}/conclusion", headers=inspector, json={"revision": 1, "outcome": "NORMAL", "description": "现场复核：未见明显表面异常。", "items": []}, timeout=10).raise_for_status()
    requests.post(f"{BASE}/observations/{oid}/submit", headers=inspector, json={"revision": 2}, timeout=10).raise_for_status()
    requests.post(f"{BASE}/observations/{oid}/review", headers=H, json={"revision": 3, "decision": "ACCEPT", "disposition": "照片与人工结论已复核"}, timeout=10).raise_for_status()
    admission_reason = "人工复核未见明显异常，不立项并保留巡检记录。"
    response = requests.post(f"{BASE}/observations/{oid}/admit", headers=H, json={"no_issue_reason": admission_reason}, timeout=10)
    response.raise_for_status()
    replay = requests.post(f"{BASE}/observations/{oid}/admit", headers=H, json={"new_issues": [{"title": "重复请求", "priority_code": "WATCH"}]}, timeout=10)
    replay.raise_for_status()
    assert replay.json()["data"]["admit"]["already_admitted"] and replay.json()["data"]["admit"]["no_issue_reason"] == admission_reason
    record = requests.get(f"{BASE}/observations/{oid}", headers=H, timeout=10).json()["data"]["evidence"]["observation"]
    assert record["admitted_at"] and record["no_issue_reason"] == admission_reason
    r = check("create task report", requests.post(f"{BASE}/reports/task/{task_id}", headers=H), 201)
    rid = r.json()["data"]["report"]["id"]
    check("get report", requests.get(f"{BASE}/reports/{rid}", headers=H))
    r = check("download pdf", requests.get(f"{BASE}/reports/{rid}/file", headers=H), 200)
    assert r.headers.get("Content-Type") == "application/pdf"
    with pymupdf.open(stream=r.content, filetype="pdf") as document:
        text = "".join(page.get_text() for page in document)
    assert "不立项" in text and admission_reason in text
    with open(OUT, "wb") as f:
        f.write(r.content)
    print(f"== PDF saved: {OUT} ({len(r.content)} bytes)")
    check("list reports", requests.get(f"{BASE}/reports?page_size=5", headers=H))
    def call(method, path, headers=H, **kwargs):
        response = requests.request(method, BASE + path, headers=headers, timeout=60, **kwargs)
        response.raise_for_status()
        return response.json()["data"]

    second = call("POST", "/tasks", json={"title": "处置进展报告验收", "assignee_id": 3, "point_ids": [point["id"]]})["task"]
    tid = second["id"]
    call("POST", f"/tasks/{tid}/dispatch", json={"revision": 1})
    second_point = call("GET", f"/tasks/{tid}")["points"][0]
    record = call("POST", "/observations", inspector, json={"source": "TASK", "task_point_id": second_point["id"], "location_text": "报告流程测试"})["observation"]
    second_oid = record["id"]
    call("POST", f"/observations/{second_oid}/images", inspector, files={"image": ("sample.png", buffer.getvalue(), "image/png")})
    call("PUT", f"/observations/{second_oid}/conclusion", inspector, json={"revision": 1, "outcome": "UNDETERMINED", "description": "仅验证软件报告", "items": []})
    call("POST", f"/observations/{second_oid}/submit", inspector, json={"revision": 2})
    call("POST", f"/observations/{second_oid}/review", json={"revision": 3, "decision": "ACCEPT", "disposition": "仅验证报告流程"})
    issue_id = call("POST", f"/observations/{second_oid}/admit", json={"new_issues": [{"title": "报告处置历史验收", "priority_code": "WATCH"}]})["admit"]["created_issue_ids"][0]
    issue = call("POST", f"/issues/{issue_id}/assign", json={"revision": 1, "assignee_id": 3})["issue"]
    progress = "处置进展联调：完成记录核对。"
    observe = "观察记录联调：等待复查。"
    issue = call("POST", f"/issues/{issue_id}/actions", inspector, json={"revision": issue["revision"], "action_type": "PROGRESS", "content": progress})["issue"]
    planned = (datetime.now(timezone.utc) + timedelta(days=1)).isoformat()
    issue = call("POST", f"/issues/{issue_id}/actions", inspector, json={"revision": issue["revision"], "action_type": "OBSERVE", "content": observe, "planned_follow_up_at": planned})["issue"]
    issue = call("POST", f"/issues/{issue_id}/request-verification", inspector, json={"revision": issue["revision"], "content": "报告联调：申请核验。"})["issue"]
    reviewer_token = call("POST", "/auth/login", json={"username": "reviewer", "password": "Review123"})["token"]
    reviewer = {"Authorization": "Bearer " + reviewer_token}
    issue = call("POST", f"/issues/{issue_id}/verify", reviewer, json={"revision": issue["revision"], "decision": "PASS", "reason": "报告联调：核验完成。"})["issue"]
    handling = call("POST", f"/reports/task/{tid}")["report"]
    handling_pdf = requests.get(f"{BASE}/reports/{handling['id']}/file", headers=H, timeout=30)
    handling_pdf.raise_for_status()
    with pymupdf.open(stream=handling_pdf.content, filetype="pdf") as document:
        handling_text = "".join(page.get_text() for page in document)
    for expected in (progress, observe, "计划复查", "巡检员", "审核员", "核验完成", "已关闭"):
        assert expected in handling_text, expected
    with open(OUT.replace(".pdf", "-handling.pdf"), "wb") as stream:
        stream.write(handling_pdf.content)
    marker = "报告生成后的重新打开操作。"
    call("POST", f"/issues/{issue_id}/reopen", reviewer, json={"revision": issue["revision"], "reason": marker})
    unchanged = requests.get(f"{BASE}/reports/{handling['id']}/file", headers=H, timeout=30)
    unchanged.raise_for_status()
    assert unchanged.content == handling_pdf.content and marker not in handling_text
    updated = call("POST", f"/reports/task/{tid}")["report"]
    updated_pdf = requests.get(f"{BASE}/reports/{updated['id']}/file", headers=H, timeout=30)
    updated_pdf.raise_for_status()
    with pymupdf.open(stream=updated_pdf.content, filetype="pdf") as document:
        assert marker in "".join(page.get_text() for page in document)
    year = datetime.now(timezone.utc).year
    ranged = check("range report", requests.post(f"{BASE}/reports/range", headers=H,
          json={"start": f"{year}-01-01T00:00:00Z", "end": f"{year}-12-31T23:59:59Z"}, timeout=60), 201).json()["data"]["report"]
    range_pdf = requests.get(f"{BASE}/reports/{ranged['id']}/file", headers=H, timeout=30)
    range_pdf.raise_for_status()
    with pymupdf.open(stream=range_pdf.content, filetype="pdf") as document:
        range_text = "".join(page.get_text() for page in document)
    assert progress in range_text and marker in range_text and admission_reason in range_text


    # 权限：巡检员无权生成/查看报告
    ti = requests.post(f"{BASE}/auth/login", json={"username": "inspector", "password": "Insp123"}).json()["data"]["token"]
    Hi = {"Authorization": "Bearer " + ti}
    check("inspector forbidden", requests.post(f"{BASE}/reports/task/1", headers=Hi), 403)
    check("inspector list forbidden", requests.get(f"{BASE}/reports", headers=Hi), 403)
    print("\nALL REPORT E2E CHECKS PASSED")


if __name__ == "__main__":
    main()
