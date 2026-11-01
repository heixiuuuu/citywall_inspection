# web/ — PC 管理端（Vue 3 + Element Plus）

> 城墙病害巡检系统的 PC 管理端：管理员派发/取消任务，审核员审核现场记录与病害结论。
> 服务端接口见 `server/api/openapi.yaml`；部署联调走 Vite 代理 `/api/v1 → 127.0.0.1:8080`。

## 技术栈

- Vue 3（`<script setup>`）、TypeScript、Vite 5、Element Plus、vue-router
- PDF.js 按需加载报告阅读器，支持翻页、缩放和适合宽度；Worker 随构建产物本地部署
- Node.js 22.13 或更高版本
- 构建产物静态站点（`dist/`），可部署到任意静态服务器

## 功能与页面

| 路由 | 页面 | 功能 | 权限 |
|---|---|---|---|
| /login | 登录 | 账号密码登录（JWT 存 localStorage） | 公开 |
| /overview | 巡检工作台 | 概况、待办与到期复查入口 | 管理员/审核员 |
| /points | 城墙档案 | 区段和点位维护、有效状态筛选 | 管理员 |
| /tasks | 任务管理 | 列表（服务端分页/状态过滤）、创建、派发、完成、取消、详情 | 查看：全部；操作：管理员/审核员 |
| /reviews | 记录审核 | 待审核记录、现场图片快照（OVERVIEW/DETAIL 分组）、接受/退回/驳回 | 审核员/管理员 |
| /records | 记录检索 | 任务、点位、时间和状态检索，证据详情及受理 | 管理员/审核员 |
| /issues | 问题台账 | 处置进展、观察安排、指派与核验 | 管理员/审核员查看全部，巡检员查看本人负责问题 |
| /reports | 报告中心 | 生成报告、在线阅读、下载及失败重试 | 管理员/审核员 |
| /audit | 审计日志 | 查询关键业务操作和状态变化 | 管理员 |

角色：`ADMIN`（管理员）/ `REVIEWER`（审核员）/ `INSPECTOR`（巡检员，主要用小程序）。
演示账号：admin/Admin123、reviewer/Review123、inspector/Insp123（由 server/cmd/seed 预置）。

## 界面与反馈

`src/styles/tokens.css` 定义颜色、间距、排版和聚焦样式。导航按巡检跟进、证据成果及基础管理分组；页面标题、状态筛选、空结果和加载失败反馈复用 `src/shared/ui` 组件。

加载失败时保留错误说明和重试入口，已有列表明确标注为上次加载内容。审核结论直接展示处理后果，保存失败保留填写内容。照片支持放大查阅，报告支持页内阅读和下载。在线阅读仅加载受控接口取得的 PDF，原件通过同一权限下载。

静态服务器必须以 JavaScript MIME 类型提供 `.mjs` 文件；Docker 中的 `nginx.conf` 已配置此规则。PDF 阅读器与 Worker 不依赖外部 CDN。

## 运行

```powershell
cd web
npm ci                 # 安装依赖
npm run dev            # 开发（http://localhost:5173，代理后端 8080）
npm run build          # 生产构建（dist/）
npx vue-tsc --noEmit   # 类型检查
```

一键启动（含后端/识别/MySQL）：仓库根 `.\scripts\dev.ps1 start`。

## 与服务端配合的约定

- 登录后 `localStorage` 保存 `city_wall_token` / `city_wall_roles`（路由守卫按角色放行）
- 图片走受控接口：`GET /api/v1/images/{id}/content`（需带 token），审核页按记录图片列表分组加载
- 任务/记录列表分页参数：`status`、`page`、`page_size`（服务端返回 `total`）
- 关键操作（派发/完成/取消/审核）均带 `revision` 乐观锁，冲突提示重新加载

## CI

`.github/workflows/ci.yml` web 作业：`npm ci` → `vue-tsc` 类型检查 → `vite build`。
