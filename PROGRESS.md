# 开发进度记录（长任务续航用）

> 本文件服务于长期任务恢复：上下文压缩/中断后，先看 Git 日志与本文件，再继续 Goal。
> 最后更新：2026-10-07

## 已完成阶段与提交哈希

| 阶段 | 内容 | 提交 |
|---|---|---|
| MVP（继承历史） | 三端骨架 + 基础巡检链路（任务/记录/图片/审核/识别 dev 响应） | 历史 feat/mvp 分支 |
| 三 | 问题全生命周期 + 报告快照 + 落库幂等 + 查询增强 + web 台账/检索/报告页 | `2dd18eb`（阶段三+四合并提交） |
| 四 | 同上（报告模块：gofpdf 中文 PDF、快照、重试、鉴权下载） | 同上 |
| 二+五 | 真实 YOLO 推理接入（坐标映射/类别映射/零检测/失败兜底）、10 关键场景整体回归、README | `阶段二+五提交`（e2e_scenarios 全绿） |
| 三端侧 | 小程序任务点真实状态 + 退回补正入口 | `42ea903` |
| 五 | OpenAPI 契约补齐 issue/report/admit 接口 | `c8218a3` + 修复提交 |

## 当前可运行命令

- Go：`cd server; go build -o bin\server.exe .\cmd\server`；`go run ./cmd/migrate -config ../conf/server.json`；`go run ./cmd/seed -config ../conf/server.json`
- Go 服务：`Start-Process bin\server.exe -WorkingDirectory C:\code\city_wall`（:8080，日志 server/server.log + server.err.log，健康检查 GET /readyz）
- 识别服务：`cd recognizer; .\.venv\Scripts\python.exe -m app.main`（:4100，读取 ../conf/recognizer.json，detector_mode=yolo 指向 data/yolo/runs/baseline001-2/weights/best.pt）
- PC：`cd web; npm run dev`（:5173）
- 联调脚本（Go 服务运行中）：`python scripts/e2e_issue.py`、`python scripts/e2e_report.py`、`python scripts/e2e_idempotency.py`、`python scripts/e2e_scenarios.py`
- MySQL：`C:\tools\mysql\mysql-8.0.42-winx64\bin\mysql.exe -u city_wall -pcity_wall_dev city_wall`（PowerShell 里 SQL 文件用 `cmd /c "mysql … < file.sql"`）
- 演示账号：admin/Admin123（ADMIN+REVIEWER）、reviewer/Review123（REVIEWER）、inspector/Insp123（INSPECTOR）

## 验证状态（阶段五回归）

`e2e_scenarios.py` 全绿，覆盖：正常巡检、模型误检（REJECT）、模型漏检（ADD）、识别失败后人工提交、单图多病害、退回补正、无任务主动上报、任务完成问题仍开放、问题核验退回后关闭、同位置多次上报关联同一问题；权限（无权读图 403）、重复提交（409）、幂等重放（200 idempotent）、并发审核冲突（409）。

真实识别链路：`POST /images/:id/recognitions` → Go → Python YOLO，attempt 9 SUCCEEDED（yolo_baseline_001），候选 CRACK conf=0.4703 坐标 (367,160)-(400,400) 在原图 400x400 内；再次识别产生新 attempt 不覆盖旧记录。识别服务 19 项测试全绿。报告 e2e 在 yolo 模式下全绿。

## YOLO 训练（后台，最终权重待确认）

- task_id：`759690e9-795d-4414-abc2-57098ad3711c`；20 epochs，imgsz=320/batch=64/patience=6，CPU
- 进度：至 2026-10-07 完成 epoch 10/20（约 7.5min/epoch，剩 ~75 分钟）；mAP50 轨迹 epoch1=0.0304 → 6=0.0647 → 7=0.0719 → 9≈0.1 区间
- 权重：`data/yolo/runs/baseline001-2/weights/best.pt`（24MB，已可加载推理；训练完成后可能更新）
- 训练完成后动作：用最终 best.pt 重跑 `recognizer` 基础效果验证（6 张 val 样例，当前全检出 CRACK、平均 1.17 候选/图、首图加载后 80-164ms CPU 推理），更新本记录与 README 中效果观察（不编造准确率）

## 实际阻塞

1. 微信正式凭据缺失：正式登录未配置，开发身份（conf/server.json auth.dev_identities）仅限开发环境；未配置时正式通道不会静默绕过登录。
2. YOLO 训练未完成前，最终 mAP 指标未知（不得编造）；当前以"基础效果观察 + 真实候选结构/坐标/权限链路"验证替代正式指标。

## 下一项具体工作

1. 等待训练完成 → 最终 best.pt 重跑基础效果验证并留档（数据在 data/yolo/ 不入库）
2. 阶段五收尾：README 补最终效果观察；检查 docs/01-06 关键场景逐项核对（已在 e2e_scenarios 覆盖 10 项）
3. 最终 Git 提交（训练留档、README 更新），完成 Goal 汇报
