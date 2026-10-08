# 《基于视觉识别的古城墙病害巡检系统设计与实现》

毕业设计项目：以 AI 辅助人工巡检的古城墙病害检测与巡检监测系统（目标对象：台州府城墙）。
设计文档见 `docs/`（01 需求与边界 → 02 业务流程 → 03 系统框架与接口 → 04 数据模型与状态 → 06 长期任务清单）。

## 仓库结构

```text
city_wall/
├── conf/                    # 服务配置（不入库密钥用环境变量覆盖）
│   ├── server.json          # Go 业务服务配置
│   └── recognizer.json      # Python 识别服务配置
├── server/                  # Go 业务服务（Gin + GORM + MySQL）
│   ├── cmd/server/          # 入口：加载配置、启动、优雅退出
│   ├── cmd/migrate/         # 按版本执行 migrations/*.sql
│   ├── cmd/seed/            # 幂等预置数据（测试账号/区段/点位）
│   ├── internal/
│   │   ├── config/          # 配置加载与环境变量覆盖
│   │   ├── bootstrap/       # 装配数据库、Service、Handler、路由
│   │   ├── api/             # HTTP Handler（参数绑定与响应，不碰 GORM）
│   │   ├── router/          # /api/v1 分组与集中注册
│   │   ├── middleware/      # auth / request_id / recovery / idempotency
│   │   ├── module/          # identity/site/task/observation/audit/issue/report/idempotency
│   │   └── infrastructure/  # database / storage / recognizer
│   ├── migrations/          # 显式 SQL 迁移（001~004，按版本执行）
│   └── api/openapi.yaml     # 对外接口契约
├── recognizer/              # Python 识别服务（FastAPI + YOLO 真实推理）
│   ├── app/
│   │   ├── main.py          # FastAPI 入口
│   │   ├── api/routes.py    # /internal/v1/recognitions
│   │   ├── schemas.py       # 检测契约（bbox / class_code / confidence）
│   │   ├── images.py        # 格式白名单 / EXIF 方向归一 / 尺寸校验
│   │   ├── service.py       # 识别编排（dev 固定响应 ↔ 真实 YOLO 隔离）
│   │   └── models/          # Detector 协议 + YOLO 适配（ultralytics）
│   └── tests/               # 契约 + 图像校验测试
├── miniapp/                 # 巡检员现场端（uni-app + Vue3，Android App 与微信小程序双端）
│   ├── package.json 等      # npm CLI 构建（build:app / build:mp-weixin）
│   └── src/pages/           # 登录/我的任务/任务详情/现场采集/主动上报
├── web/                     # PC 管理端（Vue3 + TS + Vite + Element Plus）
│   └── src/pages/           # 登录/任务/审核/问题台账/记录检索/报告中心/点位/审计
├── scripts/                 # 端到端联调脚本（e2e_issue / e2e_report / e2e_idempotency）
├── data/yolo/               # 数据集、data.yaml 与训练输出（不入库，gitignore）
├── design/miniapp/          # 小程序 UI/UE 设计
├── .github/workflows/       # CI（server/recognizer/web 三端）
└── docs/                    # 设计文档（不入库，gitignore）
```

## 快速启动（全链路）

前置：Go 1.22+、Python 3.10+、Node 18+、MySQL 8。

### 0) MySQL（本地）

Windows 免安装版：下载 `mysql-8.0.x-winx64.zip`，解压到 `C:\tools\mysql\mysql-8.0.42-winx64`，
初始化数据目录并启动：

```powershell
# 初始化（仅首次）
cd C:\tools\mysql\mysql-8.0.42-winx64
.\bin\mysqld --initialize-insecure --datadir=C:\tools\mysql\data   # root 无密码
# 启动
.\bin\mysqld --datadir=C:\tools\mysql\data --port=3306 --console
```

创建库与用户（与 `conf/server.json` 对齐）：

```sql
CREATE DATABASE IF NOT EXISTS city_wall CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'city_wall'@'localhost' IDENTIFIED BY 'city_wall_dev';
CREATE USER IF NOT EXISTS 'city_wall'@'127.0.0.1' IDENTIFIED BY 'city_wall_dev';
GRANT ALL PRIVILEGES ON city_wall.* TO 'city_wall'@'localhost';
GRANT ALL PRIVILEGES ON city_wall.* TO 'city_wall'@'127.0.0.1';
FLUSH PRIVILEGES;
```

执行迁移（按版本顺序）并预置数据：

```powershell
cd server
go run ./cmd/migrate -config ../conf/server.json     # 自动按序执行 001~004
go run ./cmd/seed   -config ../conf/server.json      # 幂等预置账号/区段/点位
```

### 1) Go 业务服务（:8080）

```powershell
cd server
$env:GOPROXY = "https://goproxy.cn,direct"
go mod tidy
go build -o bin\server.exe .\cmd\server
Start-Process .\bin\server.exe -WorkingDirectory C:\code\city_wall `
  -RedirectStandardOutput ..\server.log -RedirectStandardError ..\server.err.log
# 健康检查：curl http://127.0.0.1:8080/healthz ；MySQL 就绪后 /readyz 200
```

### 2) Python 识别服务（:4100）

```powershell
cd recognizer
python -m venv .venv && .\.venv\Scripts\Activate.ps1
pip install -r requirements.txt
# 权重放置：data/yolo/runs/baseline001-2/weights/best.pt（训练输出）
# recognizer.json 的 model.weights 指向该文件；缺权重时启动并保持 dev 模式
python -m app.main            # 从 recognizer/ 目录启动（读取 ../conf/recognizer.json）
# 契约与图像校验测试：python -m pytest tests/ -q
```

### 3) PC 管理端（:5173）

```powershell
cd web
npm install
npm run dev                   # http://localhost:5173（/api 代理到 8080）
# 演示账号：admin/Admin123（管理员+审核）、reviewer/Review123（审核员）、inspector/Insp123（巡检员）
```

### 4) 巡检员现场端（Android App 优先 + 微信小程序兼容）

uni-app（Vue3）双模式工程：`miniapp/` 既能被 HBuilderX 直接导入，也支持 npm CLI 构建。

```powershell
cd miniapp
npm install                       # 首次
npm run build:app                 # 构建 Android App 资源 → dist/build/app/（APK 前置资源）
npm run build:mp-weixin           # 构建微信小程序包 → dist/build/mp-weixin/
```

**APK 打包**（本项目默认路径：本地 Gradle 原生 WebView 壳，已验证可装机运行；HBuilderX 云打包为备选）：
- **本地打包（已验证，推荐）**：`android/` 是原生 WebView 壳工程（WebViewAssetLoader 安全加载 uni-app H5 产物），
  构建流程见下方「一键构建 APK」；产物签名在 `app/build.gradle`（开发 keystore `app/citywall-release.keystore`，
  正式发布必须更换为自己的签名）。
- **HBuilderX 云打包（备选）**：HBuilderX 导入 `miniapp/` → 登录 DCloud 账号 → manifest 可视化界面
  自动申请 uni-app AppID（`__UNI__xxx`）→ 运行 → 原生App-云打包 → Android 云端打包 → 下载 APK
  装到手机。包名/权限/图标/版本已在 `src/manifest.json` 配好（`com.citywall.inspector`、相机/定位/存储、
  `src/static/app-icon.png`、0.2.0/200）。**联调期在云打包界面取消勾选「使用安全网络」**（Android 9+
  默认禁明文 http），正式发布改为 https 并勾选。
- **离线打包**：Android Studio + DCloud 离线 SDK，将 `dist/build/app/` 放入
  `assets/apps/__UNI__<AppID>/www/`，Manifest 加 `usesCleartextTraffic="true"` 与权限，签名出包。

**一键构建 APK（本地 Gradle 壳）**：

```powershell
# 1) 构建 H5 产物（本地 WebView 壳加载；必须含 @dcloudio/uni-h5 依赖，否则产物只有统计 SDK 会白屏）
cd miniapp
npm install
npm run build:h5                # → dist/build/h5/
# 2) 同步产物到 Android assets（保持与 dist/build/h5 一致）
python -c "import os,shutil; s=r'C:\code\city_wall\miniapp\dist\build\h5'; d=r'C:\code\city_wall\android\app\src\main\assets\www'; shutil.rmtree(d) if os.path.exists(d) else None; shutil.copytree(s,d)"
# 3) 构建 APK（JDK17+，Android SDK）
$env:JAVA_HOME="<JDK17 路径>"; $env:ANDROID_HOME="<Android SDK 路径>"
cd android
gradle assembleDebug assembleRelease   # → app/build/outputs/apk/{debug,release}/
# 4) 装机测试
adb install -r app\build\outputs\apk\release\app-release.apk
adb shell am start -n com.citywall.inspector/.MainActivity
```

**真机/模拟器联调**：
- Android 模拟器（本机联调）：WebView 壳内 H5 自动使用 `http://10.0.2.2:8080/api/v1` 访问宿主机 Go 服务（见
  `src/shared/api/request.js`），**无需改代码**；Go 服务需监听 `0.0.0.0` 并启用 CORS（`server/internal/middleware/cors.go`，
  已放开跨域回显 Origin，供 WebView 的 `https://appassets.androidplatform.net` 页面调用）。
- 真机：手机与电脑同局域网，APK 访问 `http://<电脑IP>:8080/api/v1`（Go 默认监听所有网卡）；
  装机前改 `src/shared/api/request.js` 的 `BASE_URL`，或装机后
  `uni.setStorageSync('city_wall_api_base', 'http://<电脑IP>:8080/api/v1')` 覆盖（仅 App 端生效）。
- 相机/相册：现场采集页点击「拍照/从相册选择」走系统文件选择器（`MainActivity.onShowFileChooser` 已实现）。

**微信小程序兼容**：`npm run build:mp-weixin` 产物导入微信开发者工具即可；或 HBuilderX 运行到小程序。
开发模式登录用固定开发身份映射（conf/server.json `auth.dev_identities`），**仅限开发环境**；
正式微信登录需配置 `auth.wechat_app_id / wechat_app_secret`，未配置时正式通道不会静默绕过登录。

## 配置说明

- `conf/server.json`：监听、MySQL、受控存储目录（图片/PDF，客户端不可直连，经 Go 鉴权读取）、识别服务内部地址、JWT、上传约束、报告（PDF 中文字体路径，留空自动探测 `C:\Windows\Fonts\simhei.ttf`）、日志、CORS。
- `conf/recognizer.json`：模型版本、权重路径、置信度阈值、图片限额、内部鉴权 token；`mode` 区分 `dev`（固定响应，仅首轮联调）与 `yolo`（真实推理）。
- 密钥类配置支持环境变量覆盖，避免入库：`CITY_WALL_DB_DSN`、`CITY_WALL_JWT_SECRET`、`CITY_WALL_RECOGNIZER_TOKEN`、`RECOGNIZER_CONFIG`。
- 请求关联 ID：响应体含 `request_id`（请求头 `X-Request-Id` 透传或自动生成），业务日志不记录密码、token 与图片正文。

## 三端联调路径

1. PC 管理员登录 → 新建任务（选多个巡检点/负责人/期限/说明）→ 派发（保存点位快照，可幂等重放）。
2. 小程序（巡检员）登录 → 我的任务 → 逐点到场（定位+精度+概览照片）→ 图片上传到 Go 取得图片 ID → Go 调用 Python 内部识别 → 候选查看 → 人工确认/误检/补录漏检/修改框 → 任务提交。
3. PC 审核员 → 待审列表 → 审核详情（原图/定位/模型候选/人工结论/修订历史）→ 通过/退回（必须填理由）。
4. 审核通过记录 → 受理新建问题或关联已有问题；问题台账 → 指派/处理建议/进展/观察/申请核验/核验关闭或退回。
5. 报告中心 → 生成任务/时间范围 PDF 报告（快照）→ 下载；记录检索 → 按任务/时间/点位/来源/审核状态过滤，查看原图。
6. 巡检员无任务时 → 小程序主动上报（位置/照片/异常描述）→ 审核员退回补充/驳回/新建问题/关联问题，主动上报不自动完成附近巡检任务。

端到端联调脚本（服务运行中执行）：

```powershell
python scripts/e2e_issue.py          # 问题模块全链路（含权限与 revision 冲突）
python scripts/e2e_report.py         # 报告生成/下载/权限
python scripts/e2e_idempotency.py    # 派发/上传幂等重放
```

## 关键设计（保持业务闭环与数据可追溯）

- **状态机与并发**：任务/记录/问题均带 revision 乐观锁，状态变更走事务 + 条件更新（revision 过期 → 409）；重复提交/审核冲突被业务层与幂等记录双层拦截。
- **幂等**：派发/提交/审核/关闭/上传等可能重复的操作支持 `Idempotency-Key` 请求头，落库 `idempotency_records`（actor+operation+key 唯一），执行成功后才记录，失败可同键重试；图片上传幂等在 observation 服务内落库。
- **识别与人工结论**：图片先上传取得图片 ID，再触发识别；每次识别产生独立 attempt，候选带原图像素坐标（yolo.py 完成坐标映射）；模型零检测不自动等于"正常"；识别失败/超时仍可人工填写提交；误检（REJECT）/漏检（ADD）/再次识别（attempt>1）全部留存并在报告中汇总。
- **审核与上报**：审核员不能审核自己提交的记录；退回必须写明原因；主动上报保留自身来源，不自动完成附近巡检任务。
- **问题与任务独立流转**：问题有独立编号/优先级/负责人/状态；一次现场记录可支撑多个问题，一个问题可关联多次现场记录；任务完成不影响关联问题继续开放。
- **报告快照**：报告保存生成时刻的数据快照（snapshot_json + PDF 同刻生成），后续问题状态变化不修改旧报告；生成失败保留 FAILED 记录可重试，不回滚任务完成状态。
- **权限与文件**：图片与 PDF 在受控存储目录，一律经 Go 鉴权读取（`/images/:id/content`、`/reports/:id/file`），客户端不能直接访问公开静态上传目录。

## 项目现状

- 服务端：任务（多点派发/快照）、现场记录（到场/拍照/结论/提交/退回补正）、识别（开发响应与真实 YOLO 隔离，真实推理已接入）、审核（通过/退回/驳回）、问题（受理/指派/进展/观察/核验/关闭/重开）、报告（任务/范围 PDF 快照 + 重试 + 下载）、幂等、审计；`go test ./...` 全绿。
- 识别服务：FastAPI + YOLOv8n（10 类病害，训练 `data/yolo/runs/baseline001-2`），dev 固定响应与真实 YOLO 推理以 `mode` 明确隔离，不编造指标。
- PC 管理端：任务管理 / 审核管理 / 问题台账 / 记录检索 / 报告中心 / 点位管理 / 审计日志。
- 小程序/App 现场端：登录 / 我的任务 / 任务详情（真实审核状态）/ 现场采集（拍照+定位+识别确认+结论）/ 主动上报；**Android App（APK）与微信小程序双端构建已验证**（`build:app`、`build:mp-weixin` 均成功）。
- 长期事项与阻塞项见 `docs/06-长期任务清单.md`（微信正式凭据按未知处理，开发身份仅限 dev）。
