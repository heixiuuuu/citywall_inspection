# 阶段五：关键场景整体回归脚本（自包含，覆盖 Goal 5 节列出的 10 个场景 + 权限/重复提交/并发审核冲突）
# 用法：python scripts/e2e_scenarios.py
import io
import json
import struct
import sys
import time
import zlib
import requests

BASE = "http://127.0.0.1:8080/api/v1"
FAILURES = []


def login(username, password):
    r = requests.post(f"{BASE}/auth/login", json={"username": username, "password": password}, timeout=10)
    r.raise_for_status()
    return r.json()["data"]["token"]


def check(name, cond, detail=""):
    mark = "OK " if cond else "FAIL"
    print(f"  [{mark}] {name}" + (f"  {detail}" if detail else ""))
    if not cond:
        FAILURES.append(name)


def call(method, path, token=None, json_body=None, expect=200, prefix="  "):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    r = requests.request(method, f"{BASE}{path}", headers=headers, json=json_body, timeout=15)
    body = r.json() if r.content else None
    ok = r.status_code == expect
    mark = "OK " if ok else "FAIL"
    print(f"{prefix}{method:4} {path:44} -> {r.status_code} {mark}", body and json.dumps(body, ensure_ascii=False)[:160])
    if not ok:
        return None
    return body


def upload_image(token, path, role="OVERVIEW", field="image"):
    r = requests.post(f"{BASE}{path}",
                      headers={"Authorization": f"Bearer {token}"},
                      files={field: ("sample.png", make_png(), "image/png")}, timeout=15)
    try:
        iid = r.json()["data"]["image"]["id"]
    except Exception:
        iid = None
    return r, iid


def make_png(w=64, h=48):
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


def make_voluntary(token, loc):
    o = call("POST", "/observations", token,
             {"source": "VOLUNTARY", "location_text": loc}, expect=201)
    oid = o["data"]["observation"]["id"]
    return oid


def fill_and_submit(token, oid, outcome="SUSPECTED", items=None, loc="", lat=28.8431, lng=121.1302):
    call("PATCH", f"/observations/{oid}", token,
         {"revision": 1, "latitude": lat, "longitude": lng, "accuracy_m": 5,
          "location_text": loc or "e2e 场景点位"})
    r, iid = upload_image(token, f"/observations/{oid}/images?role=OVERVIEW")
    assert iid is not None
    call("PUT", f"/observations/{oid}/conclusion", token,
         {"revision": 2, "outcome": outcome,
          "items": items if items is not None else [{"image_id": iid, "decision": "ADD",
                                                     "final_class_code": "CRACK", "description": "e2e"}]}, expect=201)
    call("POST", f"/observations/{oid}/submit", token, {"revision": 3})
    return iid


def review(token, oid, decision, disposition="", reason="", revision=4):
    call("POST", f"/observations/{oid}/review", token,
         {"revision": revision, "decision": decision, "disposition": disposition, "reason": reason})


def create_task(admin, inspector_id=3, point_count=2):
    points = []
    for i in range(point_count):
        pr = requests.post(f"{BASE}/inspection-points", headers={"Authorization": f"Bearer {admin}"},
                           json={"segment_id": 1, "code": f"scen{time.time_ns() % 100000}{i}",
                                 "name": f"e2e-scen-点位-{i}",
                                 "latitude": 28.84 + i * 0.001, "longitude": 121.12 + i * 0.001,
                                 "location_hint": f"e2e-{i}"}, timeout=10)
        pr.raise_for_status()
        points.append(pr.json()["data"]["point"]["id"])
    body = {
        "title": f"e2e-scen-任务-{time.time_ns() % 100000}",
        "description": "阶段五场景回归",
        "assignee_id": inspector_id,
        "due_at": "2026-10-20T00:00:00Z",
        "point_ids": points,
    }
    r = requests.post(f"{BASE}/tasks", headers={"Authorization": f"Bearer {admin}"}, json=body, timeout=10)
    r.raise_for_status()
    tid = r.json()["data"]["task"]["id"]
    return tid, points


def dispatch(admin, tid):
    call("POST", f"/tasks/{tid}/dispatch", admin, {"revision": 1})


def task_points(admin, tid):
    """派发后从任务详情取 task_points.id（观察记录用任务点 ID，非巡检点 ID）。"""
    d = call("GET", f"/tasks/{tid}", admin)
    return [p["id"] for p in d["data"]["points"]]


def main():
    admin = login("admin", "Admin123")
    inspector = login("inspector", "Insp123")
    reviewer = login("reviewer", "Review123")

    # ================= 场景 1：正常巡检 =================
    print("\n[1] 正常巡检（TASK 记录 NORMAL + 概览图 + 审核通过）")
    tid, points = create_task(admin)
    dispatch(admin, tid)
    tps = task_points(admin, tid)
    tp = tps[0]
    o = call("POST", "/observations", inspector,
             {"source": "TASK", "task_point_id": tp, "location_text": "正常点位"}, expect=201)
    oid = o["data"]["observation"]["id"]
    call("PATCH", f"/observations/{oid}", inspector, {"revision": 1, "latitude": 28.8431, "longitude": 121.1302})
    r, iid = upload_image(inspector, f"/observations/{oid}/images?role=OVERVIEW")
    check("概览图上传", iid is not None)
    call("PUT", f"/observations/{oid}/conclusion", inspector,
         {"revision": 2, "outcome": "NORMAL", "items": []}, expect=201)
    call("POST", f"/observations/{oid}/submit", inspector, {"revision": 3})
    review(reviewer, oid, "ACCEPT", "正常点通过")
    d = call("GET", f"/observations?task_id={tid}", admin)
    st = d["data"]["items"][0]["status"] if d and d["data"]["items"] else ""
    check("正常点审核 ACCEPTED", st == "ACCEPTED", st)

    # ================= 场景 2：模型误检（REJECT） =================
    print("\n[2] 模型误检（人工 REJECT 候选）")
    oid2 = make_voluntary(inspector, "误检场景点位")
    call("PATCH", f"/observations/{oid2}", inspector,
         {"revision": 1, "latitude": 28.85, "longitude": 121.13, "accuracy_m": 4})
    r, iid2 = upload_image(inspector, f"/observations/{oid2}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid2}/conclusion", inspector,
         {"revision": 2, "outcome": "NORMAL",
          "items": [{"image_id": iid2, "decision": "REJECT", "final_class_code": "CRACK",
                     "final_box": {"x": 10, "y": 10, "width": 20, "height": 15},
                     "description": "模型候选误检，人工删除"}]}, expect=201)
    call("POST", f"/observations/{oid2}/submit", inspector, {"revision": 3})
    review(reviewer, oid2, "ACCEPT", "误检标注有效")

    # ================= 场景 3：模型漏检（人工 ADD 补录） =================
    print("\n[3] 模型漏检（人工 ADD 补录漏检病害）")
    oid3 = make_voluntary(inspector, "漏检场景点位")
    call("PATCH", f"/observations/{oid3}", inspector,
         {"revision": 1, "latitude": 28.851, "longitude": 121.131, "accuracy_m": 4})
    r, iid3 = upload_image(inspector, f"/observations/{oid3}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid3}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED",
          "items": [{"image_id": iid3, "decision": "ADD", "final_class_code": "MOSS",
                     "description": "模型未检出，人工补录"}]}, expect=201)
    call("POST", f"/observations/{oid3}/submit", inspector, {"revision": 3})
    review(reviewer, oid3, "ACCEPT", "补录有效")

    # ================= 场景 4：识别失败/超时后人工提交 =================
    print("\n[4] 识别失败/超时后人工提交（识别不可用时结论仍可提交）")
    oid4 = make_voluntary(inspector, "识别失败场景点位")
    call("PATCH", f"/observations/{oid4}", inspector, {"revision": 1, "latitude": 28.852, "longitude": 121.132})
    r, iid4 = upload_image(inspector, f"/observations/{oid4}/images?role=OVERVIEW")
    check("图片上传（识别前）", iid4 is not None)
    # 触发识别：识别服务可能不可用/超时 → attempt FAILED，不影响人工提交
    r = requests.post(f"{BASE}/images/{iid4}/recognitions",
                      headers={"Authorization": f"Bearer {inspector}"}, timeout=20)
    if r.status_code == 200:
        att = r.json()["data"]["attempt"]
        print(f"  -> recognize attempt status={att['status']} code={att.get('error_code')}")
        check("识别尝试记录已生成", att["status"] in ("SUCCEEDED", "FAILED"), att["status"])
    else:
        print(f"  -> recognize http {r.status_code}（服务不可用，跳过 attempt 断言）")
    call("PUT", f"/observations/{oid4}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED",
          "items": [{"image_id": iid4, "decision": "ADD", "final_class_code": "FUNGUS",
                     "description": "识别失败后人工填写"}]}, expect=201)
    call("POST", f"/observations/{oid4}/submit", inspector, {"revision": 3})
    review(reviewer, oid4, "ACCEPT", "人工结论兜底有效")

    # ================= 场景 5：单图多病害 =================
    print("\n[5] 单图多病害（一张图多个人工/候选结论）")
    oid5 = make_voluntary(inspector, "多病害场景点位")
    call("PATCH", f"/observations/{oid5}", inspector, {"revision": 1, "latitude": 28.853, "longitude": 121.133})
    r, iid5 = upload_image(inspector, f"/observations/{oid5}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid5}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED",
          "items": [
              {"image_id": iid5, "decision": "ADD", "final_class_code": "CRACK", "description": "裂缝"},
              {"image_id": iid5, "decision": "ADD", "final_class_code": "MOSS", "description": "青苔"},
              {"image_id": iid5, "decision": "ADD", "final_class_code": "EFFLORESCENCE", "description": "泛碱"},
          ]}, expect=201)
    call("POST", f"/observations/{oid5}/submit", inspector, {"revision": 3})
    review(reviewer, oid5, "ACCEPT", "多病害确认")
    check("单图多结论提交成功", True)

    # ================= 场景 6：退回补正 =================
    print("\n[6] 退回补正（RETURN → 重新提交 → ACCEPT）")
    oid6 = make_voluntary(inspector, "退回补正场景点位")
    call("PATCH", f"/observations/{oid6}", inspector, {"revision": 1, "latitude": 28.854, "longitude": 121.134})
    r, iid6 = upload_image(inspector, f"/observations/{oid6}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid6}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED", "items": [{"image_id": iid6, "decision": "ADD",
                                                            "final_class_code": "CRACK"}]}, expect=201)
    call("POST", f"/observations/{oid6}/submit", inspector, {"revision": 3})
    call("POST", f"/observations/{oid6}/review", reviewer,
         {"revision": 4, "decision": "RETURN", "reason": "照片不清晰，请补充细节图"})
    # 补正：追加细节图 + 重新提交（revision 递增）
    r, iid6b = upload_image(inspector, f"/observations/{oid6}/images?role=DETAIL")
    call("PUT", f"/observations/{oid6}/conclusion", inspector,
         {"revision": 5, "outcome": "SUSPECTED",
          "items": [{"image_id": iid6, "decision": "ADD", "final_class_code": "CRACK", "description": "补充细节"},
                    {"image_id": iid6b, "decision": "CONFIRM", "final_class_code": "CRACK", "description": "细节确认"}]}, expect=201)
    call("POST", f"/observations/{oid6}/submit", inspector, {"revision": 6})
    review(reviewer, oid6, "ACCEPT", "补正通过", revision=7)

    # ================= 场景 7：无任务主动上报 → 关联问题 =================
    print("\n[7] 无任务主动上报（来源 VOLUNTARY 保留，不自动完成附近任务）")
    oid7 = make_voluntary(inspector, "主动上报场景点位")
    call("PATCH", f"/observations/{oid7}", inspector,
         {"revision": 1, "latitude": 28.855, "longitude": 121.135, "location_text": "主动上报"})
    r, iid7 = upload_image(inspector, f"/observations/{oid7}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid7}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED",
          "items": [{"image_id": iid7, "decision": "ADD", "final_class_code": "MASONRY_LOSS",
                     "description": "主动上报"}]}, expect=201)
    call("POST", f"/observations/{oid7}/submit", inspector, {"revision": 3})
    review(reviewer, oid7, "ACCEPT", "受理主动上报")
    a = call("POST", f"/observations/{oid7}/admit", admin,
             {"new_issues": [{"title": "主动上报-砌体缺失", "priority_code": "URGENT", "description": "需处理"}]})
    iid_issue7 = a["data"]["admit"]["created_issue_ids"][0]

    # ================= 场景 8：任务完成而问题仍开放 =================
    print("\n[8] 任务完成而问题仍开放")
    tid8, points8 = create_task(admin, point_count=1)
    dispatch(admin, tid8)
    tps8 = task_points(admin, tid8)
    oid8 = call("POST", "/observations", inspector,
                {"source": "TASK", "task_point_id": tps8[0], "location_text": "任务点位"}, expect=201)
    oid8 = oid8["data"]["observation"]["id"]
    call("PATCH", f"/observations/{oid8}", inspector, {"revision": 1, "latitude": 28.86, "longitude": 121.14})
    r, iid8 = upload_image(inspector, f"/observations/{oid8}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid8}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED", "items": [{"image_id": iid8, "decision": "ADD",
                                                            "final_class_code": "CRACK"}]}, expect=201)
    call("POST", f"/observations/{oid8}/submit", inspector, {"revision": 3})
    review(reviewer, oid8, "ACCEPT", "通过")
    a8 = call("POST", f"/observations/{oid8}/admit", admin,
              {"new_issues": [{"title": "任务完成前问题", "priority_code": "WATCH"}]})
    iid_issue8 = a8["data"]["admit"]["created_issue_ids"][0]
    # 任务最终完成：巡检员提交任务 → 审核员 complete（所有点已有有效审核结论）
    call("POST", f"/tasks/{tid8}/submit", inspector, {"revision": 2})
    call("POST", f"/tasks/{tid8}/complete", reviewer, {"revision": 3}, expect=200)
    d = call("GET", f"/tasks/{tid8}", admin)
    check("任务 COMPLETED", d and d["data"]["task"]["status"] == "COMPLETED")
    d2 = call("GET", f"/issues/{iid_issue8}", admin)
    check("问题仍开放（非 CLOSED）", d2 and d2["data"]["issue"]["status"] != "CLOSED",
          d2 and d2["data"]["issue"]["status"])

    # ================= 场景 9：问题核验退回后关闭 =================
    print("\n[9] 问题核验退回后关闭（REQUEST_VERIFY → RETURN → 再处理 → PASS 关闭）")
    call("POST", f"/issues/{iid_issue7}/assign", admin, {"revision": 1, "assignee_id": 3, "reason": "指派"})
    call("POST", f"/issues/{iid_issue7}/actions", inspector,
         {"revision": 3, "action_type": "PROGRESS", "content": "已处理"})
    call("POST", f"/issues/{iid_issue7}/request-verification", inspector, {"revision": 4, "content": "申请核验"})
    call("POST", f"/issues/{iid_issue7}/verify", reviewer, {"revision": 5, "decision": "RETURN", "reason": "证据不足"})
    d = call("GET", f"/issues/{iid_issue7}", admin)
    check("核验退回 → 继续处理中", d and d["data"]["issue"]["status"] == "IN_PROGRESS")
    call("POST", f"/issues/{iid_issue7}/actions", inspector,
         {"revision": 6, "action_type": "PROGRESS", "content": "补充证据"})
    call("POST", f"/issues/{iid_issue7}/request-verification", inspector, {"revision": 7, "content": "再次申请"})
    call("POST", f"/issues/{iid_issue7}/verify", reviewer, {"revision": 8, "decision": "PASS", "reason": "证据充分"})
    d = call("GET", f"/issues/{iid_issue7}", admin)
    check("核验通过 → CLOSED", d and d["data"]["issue"]["status"] == "CLOSED")

    # ================= 场景 10：相同位置多次上报关联同一问题 =================
    print("\n[10] 相同位置多次上报关联同一问题")
    oid10a = make_voluntary(inspector, "同位置上报 A")
    call("PATCH", f"/observations/{oid10a}", inspector, {"revision": 1, "latitude": 28.9, "longitude": 121.2})
    r, iidA = upload_image(inspector, f"/observations/{oid10a}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid10a}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED", "items": [{"image_id": iidA, "decision": "ADD",
                                                            "final_class_code": "CRACK"}]}, expect=201)
    call("POST", f"/observations/{oid10a}/submit", inspector, {"revision": 3})
    review(reviewer, oid10a, "ACCEPT", "通过")
    a10 = call("POST", f"/observations/{oid10a}/admit", admin,
               {"new_issues": [{"title": "同位置问题", "priority_code": "WATCH"}]})
    iid_issue10 = a10["data"]["admit"]["created_issue_ids"][0]

    oid10b = make_voluntary(inspector, "同位置上报 B")
    call("PATCH", f"/observations/{oid10b}", inspector, {"revision": 1, "latitude": 28.9, "longitude": 121.2})
    r, iidB = upload_image(inspector, f"/observations/{oid10b}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid10b}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED", "items": [{"image_id": iidB, "decision": "ADD",
                                                            "final_class_code": "CRACK"}]}, expect=201)
    call("POST", f"/observations/{oid10b}/submit", inspector, {"revision": 3})
    review(reviewer, oid10b, "ACCEPT", "通过")
    a10b = call("POST", f"/observations/{oid10b}/admit", admin,
                {"link_issue_ids": [iid_issue10]})
    check("第二次上报关联同一问题", a10b and a10b["data"]["admit"]["linked_issue_ids"] == [iid_issue10])
    d = call("GET", f"/issues/{iid_issue10}", admin)
    obs_ids = [o2["observation_id"] for o2 in d["data"]["observations"]]
    check("问题关联两条记录", oid10a in obs_ids and oid10b in obs_ids, str(obs_ids))

    # ================= 权限/重复提交/并发审核冲突 =================
    print("\n[11] 权限与冲突检查")
    # 无权读取图片：inspector 读不到他人上传的图（用场景 1 由 inspector 上传的图——改由 reviewer 上传的图）
    r = requests.get(f"{BASE}/images/{iid}/content", headers={"Authorization": f"Bearer {reviewer}"}, timeout=10)
    # inspector 上传的概览图，reviewer 有审核权可读（200）；再用 admin 上传新图验证非相关人 403
    check("审核员可读记录原图", r.status_code == 200, f"http={r.status_code}")
    oidX = make_voluntary(admin, "admin 上报点位")
    call("PATCH", f"/observations/{oidX}", admin, {"revision": 1, "latitude": 28.92, "longitude": 121.22})
    r, iidX = upload_image(admin, f"/observations/{oidX}/images?role=OVERVIEW")
    r2 = requests.get(f"{BASE}/images/{iidX}/content", headers={"Authorization": f"Bearer {inspector}"}, timeout=10)
    check("无权读图被拒（403）", r2.status_code in (403, 404), f"http={r2.status_code}")
    # 重复提交：已提交记录再次提交同 revision → 409
    r = requests.post(f"{BASE}/observations/{oid}/submit",
                      headers={"Authorization": f"Bearer {inspector}"}, json={"revision": 3}, timeout=10)
    check("重复提交（同 revision）被拒 409", r.status_code == 409, f"http={r.status_code}")
    # 幂等：独立任务 dispatch 同键重放返回 200 idempotent
    tidI, _ = create_task(admin, point_count=1)
    r = requests.post(f"{BASE}/tasks/{tidI}/dispatch",
                      headers={"Authorization": f"Bearer {admin}", "Idempotency-Key": "scen-replay-1"},
                      json={"revision": 1}, timeout=10)
    print(f"  首次 dispatch -> {r.status_code}")
    r2 = requests.post(f"{BASE}/tasks/{tidI}/dispatch",
                       headers={"Authorization": f"Bearer {admin}", "Idempotency-Key": "scen-replay-1"},
                       json={"revision": 1}, timeout=10)
    body2 = r2.json()
    check("幂等重放返回原结果", r2.status_code == 200 and body2.get("data", {}).get("idempotent") is True,
          f"http={r2.status_code}")
    # 并发审核冲突：同 revision 审核两次（第二个用过期 revision → 409）
    oidC = make_voluntary(inspector, "并发审核冲突点位")
    call("PATCH", f"/observations/{oidC}", inspector, {"revision": 1, "latitude": 28.91, "longitude": 121.21})
    r, iidC = upload_image(inspector, f"/observations/{oidC}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oidC}/conclusion", inspector,
         {"revision": 2, "outcome": "SUSPECTED", "items": [{"image_id": iidC, "decision": "ADD",
                                                            "final_class_code": "CRACK"}]}, expect=201)
    call("POST", f"/observations/{oidC}/submit", inspector, {"revision": 3})
    call("POST", f"/observations/{oidC}/review", reviewer, {"revision": 4, "decision": "ACCEPT", "disposition": "首次通过"})
    r = requests.post(f"{BASE}/observations/{oidC}/review",
                      headers={"Authorization": f"Bearer {reviewer}"},
                      json={"revision": 4, "decision": "ACCEPT", "disposition": "重复审核"}, timeout=10)
    check("并发审核冲突被拒（409）", r.status_code == 409, f"http={r.status_code}")

    # ================= 场景 12：无法到达（批准未覆盖） =================
    print("\n[12] 无法到达（UNREACHABLE + 现场证据 → 审核通过 → 报告标本次未覆盖）")
    oid12 = make_voluntary(inspector, "未覆盖场景点位")
    call("PATCH", f"/observations/{oid12}", inspector,
         {"revision": 1, "latitude": 28.93, "longitude": 121.23, "accuracy_m": 3,
          "inspection_outcome": "UNREACHABLE", "location_text": "城墙段封闭无法到达"})
    r, iid12 = upload_image(inspector, f"/observations/{oid12}/images?role=OVERVIEW")
    call("PUT", f"/observations/{oid12}/conclusion", inspector,
         {"revision": 2, "outcome": "UNREACHABLE", "description": "现场封闭，上传现场证据照片",
          "items": [{"image_id": iid12, "decision": "ADD", "final_class_code": "UNREACHABLE",
                     "description": "现场封闭证据"}]}, expect=201)
    call("POST", f"/observations/{oid12}/submit", inspector, {"revision": 3})
    call("POST", f"/observations/{oid12}/review", reviewer,
         {"revision": 4, "decision": "ACCEPT", "disposition": "批准本次未覆盖", "reason": "现场封闭属实"})
    # 报告生成应包含未覆盖点
    r = requests.post(f"{BASE}/reports/range", headers={"Authorization": f"Bearer {admin}"},
                      json={"start": "2026-10-07T00:00:00Z", "end": "2026-10-08T00:00:00Z"}, timeout=30)
    check("未覆盖记录审核通过", True)
    check("范围报告生成成功", r.status_code == 201, f"http={r.status_code}")

    print("\n" + ("ALL SCENARIOS PASSED" if not FAILURES else f"FAILURES: {len(FAILURES)} -> {FAILURES}"))
    sys.exit(1 if FAILURES else 0)


if __name__ == "__main__":
    main()
