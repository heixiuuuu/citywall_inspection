# 阶段四 报告模块 API 联调：任务报告 / 范围报告 / 台账 / 下载
import requests
import sys

BASE = "http://127.0.0.1:8080/api/v1"
OUT = "report_test.pdf"


def check(name, r, expect=None):
    ok = expect is None or r.status_code == expect
    print(f"{name:24} -> {r.status_code} {'OK ' if ok else 'FAIL'} {r.text[:140]}")
    if not ok:
        sys.exit(1)
    return r


def main():
    t = requests.post(f"{BASE}/auth/login", json={"username": "admin", "password": "Admin123"})
    t.raise_for_status()
    H = {"Authorization": "Bearer " + t.json()["data"]["token"], "Content-Type": "application/json"}

    r = check("create task report", requests.post(f"{BASE}/reports/task/1", headers=H), 201)
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
