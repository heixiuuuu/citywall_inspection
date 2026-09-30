# web/ — PC 管理端（Vue 3 + Element Plus）

> 城墙病害巡检系统的 PC 管理端：管理员派发/取消任务，审核员审核现场记录与病害结论。
> 服务端接口见 `server/api/openapi.yaml`；部署联调走 Vite 代理 `/api/v1 → 127.0.0.1:8080`。

## 技术栈

- Vue 3（`<script setup>`）、TypeScript、Vite 5、Element Plus、vue-router
- 构建产物静态站点（`dist/`），可部署到任意静态服务器

## 功能与页面

| 路由 | 页面 | 功能 | 权限 |
|---|---|---|---|
| /login | 登录 | 账号密码登录（JWT 存 localStorage） | 公开 |
| /points | 点位管理 | 区段/点位列表查询 | 全部登录用户 |
| /tasks | 任务管理 | 列表（服务端分页/状态过滤）、创建、派发、完成、取消、详情 | 查看：全部；操作：管理员/审核员 |
| /reviews | 记录审核 | 待审核记录、现场图片快照（OVERVIEW/DETAIL 分组）、接受/退回/驳回 | 审核员/管理员 |

角色：`ADMIN`（管理员）/ `REVIEWER`（审核员）/ `INSPECTOR`（巡检员，主要用小程序）。
演示账号：admin/Admin123、reviewer/Review123、inspector/Insp123（由 server/cmd/seed 预置）。

## 目录结构

```
web/
├── src/
│   ├── main.ts / App.vue          # 入口与根组件
│   ├── router/index.ts            # 路由（登录守卫）
│   ├── layouts/AdminLayout.vue    # 后台布局（侧边导航）
│   ├── pages/{login,points,tasks,reviews}/index.vue
│   └── shared/
│       ├── api/{http,auth,site,task,observation}.ts   # 接口封装
│       └── utils/                 # token/角色读写
├── vite.config.ts                 # 代理 /api/v1 → 127.0.0.1:8080
├── package.json / package-lock.json
```

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
