# 阶段四 报告模块 API 联调：任务报告 / 范围报告 / 台账 / 下载
import requests
import os
import sys
import io
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
    r = check("create task report", requests.post(f"{BASE}/reports/task/{task_id}", headers=H), 201)
    rid = r.json()["data"]["report"]["id"]
    check("get report", requests.get(f"{BASE}/reports/{rid}", headers=H))
    r = check("download pdf", requests.get(f"{BASE}/reports/{rid}/file", headers=H), 200)
    assert r.headers.get("Content-Type") == "application/pdf"
    with open(OUT, "wb") as f:
        f.write(r.content)
    print(f"== PDF saved: {OUT} ({len(r.content)} bytes)")
    check("list reports", requests.get(f"{BASE}/reports?page_size=5", headers=H))
    check("range report", requests.post(f"{BASE}/reports/range",
          headers=H, json={"start": "2026-01-01T00:00:00Z", "end": "2026-12-31T23:59:59Z"}), 201)

    # 权限：巡检员无权生成/查看报告
    ti = requests.post(f"{BASE}/auth/login", json={"username": "inspector", "password": "Insp123"}).json()["data"]["token"]
    Hi = {"Authorization": "Bearer " + ti}
    check("inspector forbidden", requests.post(f"{BASE}/reports/task/1", headers=Hi), 403)
    check("inspector list forbidden", requests.get(f"{BASE}/reports", headers=Hi), 403)
    print("\nALL REPORT E2E CHECKS PASSED")


if __name__ == "__main__":
    main()
