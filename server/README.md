# 业务后端

Go / Gin / GORM / MySQL 服务。HTTP Handler 处理鉴权上下文及参数，领域 Service 处理权限与状态，Repository 处理数据库访问，基础设施处理文件及识别服务调用。

- cmd/server：HTTP 服务。
- cmd/migrate：按文件名执行 SQL，通过 schema_migrations 记录已应用版本，数据库锁避免同时执行迁移。
- cmd/seed：创建本地演示账号、区段及点位。
- internal/module：identity、site、task、observation、issue、report、audit、idempotency。
- api/openapi.yaml：HTTP 接口契约。

配置默认读取 conf/server.json，支持 CITY_WALL_CONFIG、CITY_WALL_DB_DSN、CITY_WALL_JWT_SECRET、CITY_WALL_RECOGNIZER_URL、CITY_WALL_RECOGNIZER_TOKEN、CITY_WALL_WECHAT_APP_ID、CITY_WALL_WECHAT_APP_SECRET。

从仓库根运行 `docker compose up -d --build` 自动迁移、初始化并启动。若在主机启动 Go，数据库仍使用 Docker，按配置指定映射端口，在 server 执行 `go run ./cmd/server -config ../conf/server.json`。

图片和报告经鉴权读取，不公开受控存储目录。记录提交、审核、问题处置采用 revision 冲突检测；业务测试不等于并发压力测试。数据库结构变更必须新增迁移。
