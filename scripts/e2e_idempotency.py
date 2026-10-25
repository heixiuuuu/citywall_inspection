# 阶段四 幂等联调：DB 幂等中间件（dispatch/submit/review/verify）与图片上传幂等
import requests
import os
import sys
import io
import struct
import zlib

BASE = os.getenv("CITY_WALL_API_BASE", "http://127.0.0.1:8080/api/v1")


def check(name, r, expect=None):
    ok = expect is None or r.status_code == expect
    print(f"{name:32} -> {r.status_code} {'OK ' if ok else 'FAIL'} {r.text[:120]}")
    if not ok:
        sys.exit(1)
    return r


def png_bytes(w=64, h=48):
    def chunk(t, d):
        c = struct.pack(">I", len(d)) + t + d
        return c + struct.pack(">I", zlib.crc32(t + d) & 0xFFFFFFFF)
    ihdr = struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)
    row = b"\x00" + bytes([180, 180, 180]) * w
    raw = row * h
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr)
            + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b""))


def main():
    # 管理员登录
    t = requests.post(f"{BASE}/auth/login", json={"username": "admin", "password": "Admin123"}).json()["data"]["token"]
    H = {"Authorization": "Bearer " + t, "Content-Type": "application/json", "Idempotency-Key": "idem-dispatch-1"}

    # 1) 创建任务并幂等派发（新 key，避免旧联调残留记录）
    r = requests.post(f"{BASE}/tasks", headers=H, json={
        "title": "幂等联调任务", "assignee_id": 3, "due_at": "2026-10-20T00:00:00Z",
        "point_ids": [1, 2], "note": "idem test"})
    check("create task", r, 201)
    tid = r.json()["data"]["task"]["id"]
    idem_key = f"idem-dispatch-{tid}_fresh"
    H = {"Authorization": "Bearer " + t, "Content-Type": "application/json", "Idempotency-Key": idem_key}
    r1 = requests.post(f"{BASE}/tasks/{tid}/dispatch", headers=H, json={"revision": 1, "note": "dispatch once"})
    check("dispatch first", r1, 200)
    assert r1.json()["data"].get("idempotent") is not True, "first dispatch must execute"
    r2 = requests.post(f"{BASE}/tasks/{tid}/dispatch", headers=H, json={"revision": 1, "note": "dispatch replay"})
    check("dispatch replay 200", r2, 200)
    assert r2.json()["data"].get("idempotent") is True, "replay must be marked idempotent"
    check("dispatch replay 200", r2, 200)
    assert r2.json()["data"].get("idempotent") is True, "replay must be marked idempotent"

    # 2) 同一 key 上传图片两次：第二次应无副作用（新图片不落库）
    hi = {"Authorization": "Bearer " + requests.post(f"{BASE}/auth/login",
          json={"username": "inspector", "password": "Insp123"}).json()["data"]["token"]}
    r = requests.post(f"{BASE}/tasks/{tid}", headers=hi)
    # 任务没有点位现场记录时用主动上报验证图片幂等
    r = requests.post(f"{BASE}/observations", headers=hi, json={"source": "VOLUNTARY", "location_text": "idem 位置"})
    check("create voluntary obs", r, 201)
    obs_id = r.json()["data"]["observation"]["id"]
    u1 = requests.post(f"{BASE}/observations/{obs_id}/images", headers={
        "Authorization": hi["Authorization"], "Idempotency-Key": f"idem-img-{obs_id}"},
        files={"image": ("a.png", png_bytes(), "image/png")}, data={"role": "OVERVIEW"})
    check("upload image first", u1, 201)
    img_id = u1.json()["data"]["image"]["id"]
    u2 = requests.post(f"{BASE}/observations/{obs_id}/images", headers={
        "Authorization": hi["Authorization"], "Idempotency-Key": f"idem-img-{obs_id}"},
        files={"image": ("a.png", png_bytes(), "image/png")}, data={"role": "OVERVIEW"})
    check("upload image replay", u2, 200)
    assert u2.json()["data"].get("replayed") is True
    # 确认图片数量未增加
    r = requests.get(f"{BASE}/observations/{obs_id}/images", headers=hi)
    imgs = r.json()["data"]["images"]
    assert len(imgs) == 1 and imgs[0]["id"] == img_id, f"duplicate image created: {imgs}"

    print("\nALL IDEMPOTENCY E2E CHECKS PASSED")


if __name__ == "__main__":
    main()
