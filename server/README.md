# server/ — 城墙巡检业务服务端（Go）

> 城墙病害巡检系统的后端：鉴权、任务/记录/点位领域服务、受控图片存储、审计与识别调用。
> 设计文档见 `docs/01~06`；接口契约见 `server/api/openapi.yaml`。

## 技术栈与依赖

- Go 1.27+、Gin（HTTP 框架）、GORM（ORM）、MySQL 8（业务库 `city_wall` / 测试库 `city_wall_test`）
- JWT 鉴权（`authn`）、bcrypt 口令哈希；图片本地磁盘存储（`storage`，路径由 conf/server.json 配置）

## 目录结构

```
server/
├── api/                    # HTTP 层：openapi.yaml（接口契约）+ handler（gin）
├── cmd/
│   ├── migrate/            # 迁移执行器（migrations/001~003）
│   ├── seed/               # 幂等预置：账号/区段/点位
│   └── server/             # 服务入口（-config ..\conf\server.json）
├── conf/server.json        # 运行配置（库连接/存储/识别/鉴权/上传）
├── internal/
│   ├── api/                # 各模块 handler（identity/site/task/observation）
│   ├── authn/              # TokenManager（签发/校验 JWT）
│   ├── bootstrap/          # 依赖装配（repo→service→handler）
│   ├── config/             # 配置加载（文件 + 环境变量覆盖）
│   ├── infrastructure/
│   │   ├── database/       # GORM 连接
│   │   ├── recognizer/     # 识别服务客户端（HTTP）
│   │   └── storage/        # 受控图片存储（SaveImage/Open）
│   ├── middleware/         # 鉴权/角色解析
│   └── module/
│       ├── audit/          # 审计事件记录器（落库 audit_events，失败不阻断）
│       ├── identity/       # 用户/登录（PC 账号 + 微信测试号映射）
│       ├── observation/    # 现场记录：图片/识别/结论/提交/审核
│       ├── site/           # 区段与点位
│       └── task/           # 巡检任务与任务点（状态机 + 乐观锁）
└── migrations/             # 001_init / 002_seed / 003_audit 等 SQL
```

## 运行

```powershell
# 迁移 + seed（幂等，可重复执行）
cd server
go run ./cmd/migrate -config ..\conf\server.json -dir migrations
go run ./cmd/seed -config ..\conf\server.json

# 启动服务（监听 :8080，健康检查 /readyz）
go run ./cmd/server/main.go -config ..\conf\server.json
```

一键启动（含 MySQL/识别服务/PC 管理端）：仓库根执行 `.\scripts\dev.ps1 start`。

## 配置（conf/server.json）

| 段 | 关键项 | 说明 |
|---|---|---|
| database | host/port/user/password/dbname | MySQL 连接（dev：city_wall/city_wall_dev） |
| auth | jwt_secret / token_ttl_hours | JWT 密钥与有效期（72h） |
| auth.wechat_* | wechat_app_id / wechat_app_secret | 微信测试号（AppID 已配置，Secret 待填） |
| storage | image_dir / report_dir / max_image_bytes | 图片与报告存储 |
| recognizer | base_url / internal_token | 识别服务地址（4100） |
| upload | idempotency_key_header | 上传幂等键请求头（Idempotency-Key） |

环境变量覆盖：`CITY_WALL_CONFIG`（配置文件路径）、`CITY_WALL_DB_DSN`（库 DSN）、`CITY_WALL_TEST_DSN`（集成测试 DSN）。

## 测试

```powershell
cd server
# 单元测试（不依赖 MySQL）
go test ./... -count=1

# 集成测试（真实 MySQL city_wall_test 库，TestMain 自动执行迁移）
go test -tags integration ./internal/module/task/ ./internal/module/observation/ -count=1
```

CI（.github/workflows/ci.yml）：go vet / build / test + MySQL 服务容器集成测试 + recognizer pytest + web 构建。

## 主要接口（详见 openapi.yaml）

- `POST /api/v1/auth/login`、`POST /api/v1/auth/wechat-login` — 登录
- `GET/POST /api/v1/tasks`、`POST /tasks/{id}/dispatch|submit|complete|cancel` — 任务闭环
- `GET/POST /api/v1/observations`、`POST /observations/{id}/images`、`GET /observations/{id}/images`、`PUT .../conclusion`、`POST .../review` — 现场记录闭环
- `GET /api/v1/images/{id}/content` — 受控图片流
- `GET /api/v1/segments`、`GET /api/v1/points` — 区段/点位

## 状态机与并发约定

- 任务：DRAFT → ASSIGNED → IN_PROGRESS → PENDING_REVIEW →（RETURNED↺）→ COMPLETED / CANCELLED
- 记录：DRAFT → SUBMITTED → ACCEPTED / RETURNED / REJECTED
- 所有状态变更走乐观锁（revision），冲突返回 409；关键操作写入 `audit_events`。
