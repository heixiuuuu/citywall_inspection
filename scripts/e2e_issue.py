# 阶段三 Issue 模块端到端 API 联调脚本（自包含：记录→审核→受理→处置→退回→关闭）
# 用法：python scripts/e2e_issue.py
import io
import json
import sys
import time

import requests
import os

BASE = os.getenv("CITY_WALL_API_BASE", "http://127.0.0.1:8080/api/v1")


def login(username, password):
    r = requests.post(f"{BASE}/auth/login", json={"username": username, "password": password}, timeout=10)
    r.raise_for_status()
    return r.json()["data"]["token"]


def call(method, path, token=None, json_body=None, expect=200, prefix=""):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    r = requests.request(method, f"{BASE}{path}", headers=headers, json=json_body, timeout=10)
    body = r.json() if r.content else None
    ok = r.status_code == expect
    mark = "OK " if ok else "FAIL"
    print(f"{prefix}{method:4} {path:48} -> {r.status_code} {mark}", body and json.dumps(body, ensure_ascii=False)[:220])
    if not ok:
        sys.exit(1)
    return body


def make_png():
    # 最简合法 PNG（64x48 灰底带深色矩形）
    import struct
    import zlib

    w, h = 64, 48
    raw = bytearray()
    for y in range(h):
        raw.append(0)
        for x in range(w):
            if 20 <= y <= 34 and 24 <= x <= 44:
                raw += bytes((80, 70, 60))
            else:
                raw += bytes((180, 178, 170))
    def chunk(tag, data):
        c = tag + data
        return struct.pack(">I", len(data)) + c + struct.pack(">I", zlib.crc32(c) & 0xFFFFFFFF)
    ihdr = struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", zlib.compress(bytes(raw))) + chunk(b"IEND", b"")
    return io.BytesIO(png)


def main():
    admin = login("admin", "Admin123")
    inspector = login("inspector", "Insp123")
    reviewer = login("reviewer", "Review123")

    # ---- 1. 主动上报记录：创建 → 坐标 → 图片 → 结论 → 提交 ----
    o = call("POST", "/observations", inspector,
             {"source": "VOLUNTARY", "location_text": "e2e 受理链路测试点 A"}, expect=201)
    oid = o["data"]["observation"]["id"]
    rev = o["data"]["observation"]["revision"]
    call("PATCH", f"/observations/{oid}", inspector,
         {"revision": rev, "latitude": 28.8431, "longitude": 121.1302, "accuracy_m": 5})
    # 上传图片（multipart）
    r = requests.post(f"{BASE}/observations/{oid}/images?role=OVERVIEW",
                      headers={"Authorization": f"Bearer {inspector}"},
                      files={"image": ("sample.png", make_png(), "image/png")}, timeout=10)
    r.raise_for_status()
    iid = r.json()["data"]["image"]["id"]
    print(f"POST /observations/{oid}/images        -> {r.status_code} OK  image_id={iid}")
    rev2 = 2
    call("PUT", f"/observations/{oid}/conclusion", inspector,
         {"revision": rev2, "outcome": "SUSPECTED",
          "items": [{"image_id": iid, "decision": "ADD", "final_class_code": "CRACK",
                     "final_box": {"x": 10, "y": 10, "width": 20, "height": 15}, "description": "e2e 裂缝"}]}, expect=201)
    call("POST", f"/observations/{oid}/submit", inspector, {"revision": 3})

    # ---- 2. 审核通过 ----
    call("POST", f"/observations/{oid}/review", reviewer,
         {"revision": 4, "decision": "ACCEPT", "disposition": "确认疑似病害，受理立项"})

    # ---- 3. 受理：新建问题 ----
    a = call("POST", f"/observations/{oid}/admit", admin,
             {"new_issues": [{"title": "e2e 受理问题-裂缝", "priority_code": "PROCESS",
                              "description": "联调自动创建", "priority_reason": "需处理"}]})
    iid_issue = a["data"]["admit"]["created_issue_ids"][0]
    assert a["data"]["admit"]["linked_issue_ids"] == []
    print(f"== created issue#{iid_issue}")

    # 幂等：重复受理返回已受理，不重复建问题
    a2 = call("POST", f"/observations/{oid}/admit", admin,
              {"new_issues": [{"title": "重复问题", "priority_code": "WATCH"}]})
    assert a2["data"]["admit"]["already_admitted"] is True

    # ---- 4. 处置主链路：指派 → 进展 → 申请核验 → 核验通过关闭 ----
    call("POST", f"/issues/{iid_issue}/assign", admin, {"revision": 1, "assignee_id": 3, "reason": "指派巡检员"})
    call("POST", f"/issues/{iid_issue}/actions", inspector,
         {"revision": 3, "action_type": "PROGRESS", "content": "已做封缝处理"})
    call("POST", f"/issues/{iid_issue}/request-verification", inspector, {"revision": 4, "content": "请核验"})
    call("POST", f"/issues/{iid_issue}/verify", reviewer, {"revision": 5, "decision": "PASS", "reason": "核验通过"})
    d = call("GET", f"/issues/{iid_issue}", admin)
    assert d["data"]["issue"]["status"] == "CLOSED"
    assert d["data"]["issue"]["closed_at"] and d["data"]["issue"]["close_reason"] == "核验通过"
    print(f"== issue#{iid_issue} CLOSED（主链路完成）")

    # ---- 5. 重开 + 退回链路：reopen → 进展 → 申请 → RETURN 退回 → 再进展 → PASS 关闭 ----
    call("POST", f"/issues/{iid_issue}/reopen", reviewer, {"revision": 6, "reason": "复查发现扩展"})
    reopened = call("GET", f"/issues/{iid_issue}", admin)["data"]["issue"]
    assert reopened["closed_at"] is None and reopened["close_reason"] == ""
    call("POST", f"/issues/{iid_issue}/actions", inspector,
         {"revision": 7, "action_type": "PROGRESS", "content": "二次处理"})
    call("POST", f"/issues/{iid_issue}/request-verification", inspector, {"revision": 8, "content": "二次申请"})
    # 提交人自核验被拒（inspector 非审核员 → 403）
    call("POST", f"/issues/{iid_issue}/verify", inspector, {"revision": 9, "decision": "PASS", "reason": ""}, expect=403)
    call("POST", f"/issues/{iid_issue}/verify", reviewer, {"revision": 9, "decision": "RETURN", "reason": "证据不足"})
    d2 = call("GET", f"/issues/{iid_issue}", admin)
    assert d2["data"]["issue"]["status"] == "IN_PROGRESS", d2["data"]["issue"]["status"]
    print(f"== issue#{iid_issue} RETURN → IN_PROGRESS")
    call("POST", f"/issues/{iid_issue}/actions", inspector,
         {"revision": 10, "action_type": "PROGRESS", "content": "补充证据"})
    call("POST", f"/issues/{iid_issue}/request-verification", inspector, {"revision": 11, "content": "再次申请"})
    call("POST", f"/issues/{iid_issue}/verify", reviewer, {"revision": 12, "decision": "PASS", "reason": "证据充分"})
    d3 = call("GET", f"/issues/{iid_issue}", admin)
    assert d3["data"]["issue"]["status"] == "CLOSED"
    assert d3["data"]["issue"]["closed_at"] and d3["data"]["issue"]["close_reason"] == "证据充分"
    print(f"== issue#{iid_issue} CLOSED（退回后重新关闭）")

    # ---- 6. 权限与冲突 ----
    call("POST", f"/issues/{iid_issue}/assign", inspector, {"revision": 999, "assignee_id": 3, "reason": "x"}, expect=403)
    call("POST", f"/issues/{iid_issue}/actions", inspector,
         {"revision": 9999, "action_type": "PROGRESS", "content": "x"}, expect=409)
    call("GET", "/issues?page_size=10", inspector)
    call("GET", f"/issues/{iid_issue}", inspector)  # 责任人可见详情
    print("== 权限与 revision 冲突检查 OK")

    print("\nALL ISSUE E2E CHECKS PASSED")


if __name__ == "__main__":
    main()
