# recognizer/ — 病害识别服务（FastAPI）

> 城墙病害图像识别服务：接收服务端图片，做格式/EXIF 校验，调用检测器返回病害候选框。
> 当前为契约骨架（FixedDetector 固定响应），YOLO 真实推理待训练权重接入。

## 技术栈与依赖

- Python 3.12+、FastAPI、Uvicorn、Pillow、pydantic
- 依赖（轻量，无 torch）：`requirements.txt`；模型接入阶段按需追加 torch/ultralytics

## 目录结构

```
recognizer/
├── app/
│   ├── main.py        # FastAPI 入口（/healthz /detect）
│   ├── config.py      # 配置（模型路径、阈值、超时）
│   ├── images.py      # 图像校验：格式白名单 / EXIF 方向归一 / 损坏拒绝
│   ├── schemas.py     # 请求/响应契约
│   ├── service.py     # 检测编排（校验 → 检测器 → 坐标回映）
│   ├── api/routes.py  # 路由（internal_token 校验）
│   └── models/
│       ├── protocol.py # 检测器接口协议
│       ├── fixed.py    # FixedDetector（骨架：固定响应，契约测试用）
│       └── yolo.py     # YOLODetector（Ultralytics 推理，待权重接入）
├── tests/             # pytest 契约测试 + 图像校验测试
├── Dockerfile         # 容器化（构建阶段）
├── requirements.txt
└── .venv/             # 本地虚拟环境（不入库）
```

## 运行

```powershell
cd recognizer
python -m venv .venv
.\.venv\Scripts\pip install -r requirements.txt
.\.venv\Scripts\python -m uvicorn app.main:app --host 127.0.0.1 --port 4100
```

或一键：仓库根 `.\scripts\dev.ps1 start`（自动使用 .venv 启动）。

## 契约

- `GET /healthz` → `{"status":"ok","model_version":"..."}`
- `POST /detect`（multipart: file, request_id, image_id, internal_token）→ 检测结果（候选框/置信度/延迟）

失败码：`INVALID_IMAGE`（422，含原因）、`MODEL_UNAVAILABLE`（503，模型未加载）、`UNAUTHORIZED`（401）。

## 测试

```powershell
cd recognizer
.\.venv\Scripts\python -m pytest tests/ -q    # 契约 + 图像校验全部用例
```

## 模型接入（真实 YOLO）

1. 训练产出 `best.pt`（数据集 `data/yolo/`，训练命令见 `data/yolo/README.md`；本机不训练，云端/GPU 执行）
2. 放置权重：`recognizer/models/yolo_baseline_001.pt`
3. 配置 `app/config.py`：`model_path`、`detector_type: fixed → yolo`
4. `app/models/yolo.py` 完成推理：Ultralytics 加载、EXIF 归一、网络输入尺寸框坐标回映原图、置信度过滤
5. 回归：契约测试不回归 + 真实图片返回坐标候选；`MODEL_UNAVAILABLE` 路径有测试

## 与整体架构

- 服务端（server 8080）通过 `conf/server.json recognizer.base_url`（4100）调用本服务
- 识别结果由服务端持久化为 `recognition_attempts` / `detections`，人工审核引用为结论候选
