# 《基于视觉识别的古城墙病害巡检系统设计与实现》

毕业设计项目：以 AI 辅助人工巡检的古城墙病害检测与巡检监测系统（目标对象：台州府城墙）。
设计文档见 `docs/`（01 需求与边界 → 02 业务流程 → 03 系统框架与接口 → 04 数据模型与状态）。

## 仓库结构

```text
city_wall/
├── conf/                    # 服务配置（不入库密钥用环境变量覆盖）
│   ├── server.json          # Go 业务服务配置
│   └── recognizer.json      # Python 识别服务配置
├── server/                  # Go 业务服务（Gin + GORM + MySQL）
│   ├── cmd/server/          # 入口：加载配置、启动、优雅退出
│   ├── internal/
│   │   ├── config/          # 配置加载与环境变量覆盖
│   │   ├── bootstrap/       # 装配数据库、Service、Handler、路由
│   │   ├── api/             # HTTP Handler（参数绑定与响应，不碰 GORM）
│   │   ├── router/          # /api/v1 分组与集中注册
│   │   ├── middleware/      # auth / request_id / recovery
│   │   ├── module/          # task、observation、recognition、review、issue…
│   │   └── infrastructure/  # database / storage / recognizer / pdf
│   ├── migrations/          # 显式 SQL 迁移（按版本执行）
│   └── api/openapi.yaml     # 对外接口契约
├── recognizer/              # Python 识别服务（FastAPI + YOLO，内部接口）
│   ├── app/
│   │   ├── main.py          # FastAPI 入口
│   │   ├── api/routes.py    # /internal/v1/recognitions
│   │   ├── schemas.py       # 检测契约（bbox / class_code / confidence）
│   │   ├── service.py       # 识别编排
│   │   └── models/          # Detector 协议 + YOLO 适配（当前固定响应）
│   └── tests/               # 契约测试
├── miniapp/                 # 微信小程序（uni-app + Vue3，巡检员现场端）
├── web/                     # PC 管理端（Vue3 + TS + Vite + Element Plus）
├── data/                    # 数据集与参考素材（不入库，gitignore）
└── docs/                    # 设计文档（不入库，gitignore）
```

## 快速启动

前置：Go 1.27+、Python 3.10+、Node 18+、MySQL 8（可后装）。

```bash
# 1) Go 业务服务（dev 阶段 MySQL 未就绪也可降级启动，/readyz 返回 503）
cd server
go mod tidy
go run ./cmd/server -config ../conf/server.json
# 健康检查：curl http://127.0.0.1:8080/healthz

# 2) Python 识别服务（当前固定响应验证契约）
cd recognizer
pip install -r requirements.txt
python -m app.main            # 从 recognizer/ 目录启动
# 契约测试：python -m pytest tests/ -q

# 3) PC 管理端
cd web
npm install && npm run dev    # http://localhost:5173

# 4) 微信小程序：用 HBuilderX 打开 miniapp/ 运行到微信开发者工具
```

## 配置说明

- `conf/server.json`：Go 业务服务配置（监听、MySQL、受控存储目录、识别服务地址、JWT、上传约束、日志、CORS）。
- `conf/recognizer.json`：识别服务配置（模型版本、权重路径、置信度阈值、图片限额、内部鉴权）。
- 密钥类配置支持环境变量覆盖，避免入库：`CITY_WALL_DB_DSN`、`CITY_WALL_JWT_SECRET`、`CITY_WALL_RECOGNIZER_TOKEN`、`RECOGNIZER_CONFIG`。
- 数据库迁移按版本执行：`mysql -u <user> -p <dbname> < server/migrations/001_init.sql`。

## 初始化状态

- 本次提交为项目初始化：确立仓库分层与接口约定，各端搭建可编译骨架；
- Go 服务可启动并通过 `/healthz`；业务接口（任务、记录）当前返回 `NOT_IMPLEMENTED`，待按首条纵向链路实现；
- 识别服务先用固定响应验证契约，再接真实 YOLO（`recognizer/app/models/yolo.py`）；
- 后续迁移：002 任务与现场记录、003 识别与人工结论、004 问题与处置、005 报告与幂等。
