# 开发进度记录（长任务续航用）

> 本文件服务于长期任务恢复：上下文压缩/中断后，先看 Git 日志与本文件，再继续 Goal。
> 最后更新：2026-10-08

## 已完成阶段与提交哈希

| 阶段 | 内容 | 提交 |
|---|---|---|
| MVP（继承历史） | 三端骨架 + 基础巡检链路（任务/记录/图片/审核/识别 dev 响应） | 历史 feat/mvp 分支 |
| 三 | 问题全生命周期 + 报告快照 + 落库幂等 + 查询增强 + web 台账/检索/报告页 | `2dd18eb`（阶段三+四合并提交） |
| 四 | 同上（报告模块：gofpdf 中文 PDF、快照、重试、鉴权下载） | 同上 |
| 二+五 | 真实 YOLO 推理接入（坐标映射/类别映射/零检测/失败兜底）、10 关键场景整体回归、README | `c8c773d`（e2e_scenarios 全绿） |
| 三端侧 | 小程序任务点真实状态 + 退回补正入口 | `42ea903` |
| 五 | OpenAPI 契约补齐 issue/report/admit 接口 | `c8218a3` + `963ebfe` 语法修复 |
| 五 | UNREACHABLE 未覆盖场景回归 | `c68e55f` |
| 收尾 | 进度记录、YOLO 口径调整 | `16a9062` + 本文件后续提交 |

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

## YOLO 模型（极小量可行性验证，按最新 Goal 不需要全量训练）

- 训练按新 Goal 口径停止（task `759690e9-795d-4414-abc2-57098ad3711c`，运行至 epoch 17/20 时主动终止，不再需要全量完整训练）
- 可行性验证（极小量数据 + 已训练的中间 best 权重）：
  - 权重：`data/yolo/runs/baseline001-2/weights/best.pt`（24MB，epoch 14 best，训练结果 mAP50≈0.134、mAP50-95≈0.069，仅作可行性参考，不当作成品指标）
  - 6 张 val 样例图真实推理全检出（brick 样本 CRACK 类），每图 1-3 个候选、平均 1.33；首图含模型加载 3908ms，之后 CPU 推理 27-43ms
  - 真实链路联调：`POST /images/:id/recognitions` → Go → Python YOLO，attempt 9 SUCCEEDED（yolo_baseline_001），候选 CRACK conf=0.4703 坐标 (367,160)-(400,400) 在原图 400x400 内；再次识别产生新 attempt 不覆盖旧记录
  - 识别服务 19 项测试全绿（坐标映射/阈值/零检测/缺权重/EXIF）
  - 不编造准确率：训练与验证指标以上述真实观测为限

## 实际阻塞

1. 微信正式凭据缺失：正式登录未配置，开发身份（conf/server.json auth.dev_identities）仅限开发环境；未配置时正式通道不会静默绕过登录。
2. YOLO 按新 Goal 口径只做极小量可行性验证（已通过，epoch 17 后主动终止全量训练）；训练过程指标（mAP50≈0.134）仅为基础观测，非成品准确率，不编造。

## 下一项具体工作（已完成，无遗留）

1. ✅ 极小量可行性验证留档（当前 best.pt 对 6 张 val 样例全检出，坐标映射正确）
2. ✅ 阶段五收尾：README/PROGRESS 更新、最终 Git 提交
3. ✅ 最终 Goal 汇报（区分已实现/已验证/仍受阻）
