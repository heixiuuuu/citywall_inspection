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
│   ├── cmd/seed/            # 幂等预置数据（测试账号/区段/点位）
│   ├── internal/
│   │   ├── config/          # 配置加载与环境变量覆盖
│   │   ├── bootstrap/       # 装配数据库、Service、Handler、路由
│   │   ├── api/             # HTTP Handler（参数绑定与响应，不碰 GORM）
│   │   ├── router/          # /api/v1 分组与集中注册
│   │   ├── middleware/      # auth / request_id / recovery
│   │   ├── module/          # identity/site/task/observation/audit
│   │   └── infrastructure/  # database / storage / recognizer
│   ├── migrations/          # 显式 SQL 迁移（001~003，按版本执行）
│   └── api/openapi.yaml     # 对外接口契约 v0.2.0
├── recognizer/              # Python 识别服务（FastAPI + 图像校验 + YOLO 占位）
│   ├── app/
│   │   ├── main.py          # FastAPI 入口
│   │   ├── api/routes.py    # /internal/v1/recognitions
│   │   ├── schemas.py       # 检测契约（bbox / class_code / confidence）
│   │   ├── images.py        # 格式白名单 / EXIF 方向归一 / 尺寸校验
│   │   ├── service.py       # 识别编排
│   │   └── models/          # Detector 协议 + YOLO 适配（当前固定响应）
│   └── tests/               # 契约 + 图像校验测试（12 用例）
├── miniapp/                 # 微信小程序（uni-app + Vue3，巡检员现场端）
│   └── src/pages/           # 登录/我的任务/任务详情/现场采集/主动上报
├── web/                     # PC 管理端（Vue3 + TS + Vite + Element Plus）
│   └── src/pages/           # 登录/任务管理/审核管理/点位管理
├── design/miniapp/          # 小程序 UI/UE 设计（README/ui-spec/高保真原型）
├── .github/workflows/       # GitHub Actions CI（server/recognizer/web 三端）
├── data/                    # 数据集与参考素材（不入库，gitignore）
└── docs/                    # 设计文档（不入库，gitignore）
```

## 快速启动（全链路）

前置：Go 1.27+、Python 3.10+、Node 18+、MySQL 8。

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
mysql -u city_wall -pcity_wall_dev city_wall < server/migrations/001_init.sql
mysql -u city_wall -pcity_wall_dev city_wall < server/migrations/002_mvp_tasks_observations.sql
mysql -u city_wall -pcity_wall_dev city_wall < server/migrations/003_mvp_recognition_conclusion.sql
cd server && go run ./cmd/seed -config ../conf/server.json
```

### 1) Go 业务服务

```powershell
cd server
$env:GOPROXY = "https://goproxy.cn,direct"
go mod tidy
go run ./cmd/server -config ../conf/server.json
# 健康检查：curl http://127.0.0.1:8080/healthz
# MySQL 就绪后 /readyz 返回 200；未就绪按设计降级返回 503
```

### 2) Python 识别服务

```powershell
cd recognizer
pip install -r requirements.txt
python -m app.main            # 从 recognizer/ 目录启动（读取 ../conf/recognizer.json）
# 契约与图像校验测试：python -m pytest tests/ -q（12 用例）
```

### 3) PC 管理端

```powershell
cd web
npm install
npm run dev                   # http://localhost:5173（/api 代理到 8080）
# 演示账号：admin / Admin123（管理员，兼审核）、reviewer / Review123（审核员）
```

### 4) 微信小程序

用 HBuilderX 打开 `miniapp/`（uni-app Vue3 工程），运行到微信开发者工具；
MVP 阶段登录用固定 code 映射（test-admin / test-reviewer / test-inspector），
真实微信登录需在 `conf/server.json` 配置 `auth.wechat_app_id / wechat_app_secret` 后联调。

## 配置说明

- `conf/server.json`：Go 业务服务配置（监听、MySQL、受控存储目录、识别服务地址、JWT、上传约束、日志、CORS）。
- `conf/recognizer.json`：识别服务配置（模型版本、权重路径、置信度阈值、图片限额、内部鉴权）。
- 密钥类配置支持环境变量覆盖，避免入库：`CITY_WALL_DB_DSN`、`CITY_WALL_JWT_SECRET`、`CITY_WALL_RECOGNIZER_TOKEN`、`RECOGNIZER_CONFIG`。
- 迁移显式按版本执行（见 0 节）；新增迁移继续追加 004/005…。

## 项目现状（MVP 全链路已可用）

- 服务端 MVP 纵向链路：身份（JWT/微信 MVP 映射）→ 任务（状态机+乐观锁 revision）→ 现场记录（DRAFT→SUBMITTED 五步链）→ 识别调用 → 人工结论 → 审核（通过/退回/驳回），契约见 `server/api/openapi.yaml` v0.2.0；
- 服务端测试：authn/task/observation 单元 + HTTP 集成（内存仓储），MySQL 就绪后补 GORM 集成测试（todo P0）；
- 识别服务：契约 + 图像校验测试 12 用例全绿；真实 YOLO 权重接入为 todo P0（需台州府城墙病害训练权重）；
- 小程序：按 `design/miniapp/` UI/UE 设计稿实现五页（登录/我的任务/任务详情/现场采集/主动上报）；
- PC 管理端：任务管理 / 审核管理 / 点位管理，路由守卫按角色显隐；
- CI：GitHub Actions 三端验证（push 自动运行），MySQL 集成测试容器预留。
- 长期事项与阻塞项见 `docs/06-长期任务清单.md`。
