# miniapp/ — 微信小程序端（uni-app Vue 3）

> 城墙病害巡检小程序：巡检员在城墙现场采集病害（拍照→识别→结论→提交），任务闭环。
> UI/UE 设计稿见 `design/`（先设计后开发的流程产出）；服务端接口见 `server/api/openapi.yaml`。

## 技术栈

- uni-app（Vue 3 语法）、微信小程序平台；HBuilderX 导入 / 微信开发者工具运行
- 轻量自研请求封装（`src/shared/api`），JWT 鉴权与 PC 端一致

## 页面（src/pages）

| 页面 | 功能 | 依赖接口 |
|---|---|---|
| index | 登录（微信 code → 后端换取 JWT） | `/auth/wechat-login` |
| tasks | 我的巡检任务（负责人视角） | `GET /tasks` |
| collect | 现场采集：创建记录、拍照上传（OVERVIEW/DETAIL）、识别、填结论 | `/observations`、`/observations/{id}/images`、`/images/{id}/recognitions`、`.../conclusion` |
| report | 上报/提交与状态查看 | `POST /observations/{id}/submit` |

## 运行

1. 微信开发者工具导入 `miniapp/`（或 HBuilderX 导入后运行到微信小程序）
2. 小程序后台：测试号 AppID `wx72b5ed580b73500a`（项目已配置到服务端 `conf/server.json`；
   **AppSecret 需在小程序测试号管理页获取后填入**，用于真实登录联调）
3. 后端地址：小程序 request 合法域名需配置（本地联调可勾选“不校验合法域名”）

登录身份：`auth.wechat_app_id` + 临时 code；MVP 阶段服务端保留测试身份映射
（code `test-inspector` / `test-reviewer` / `test-admin`）。

## 目录结构

```
miniapp/
├── src/
│   ├── App.vue / main.js / manifest.json / pages.json   # 入口与页面注册
│   ├── pages/{index,tasks,collect,report}/              # 4 个页面
│   ├── shared/
│   │   ├── api/                                         # 接口封装
│   │   ├── auth/                                        # 登录态
│   │   └── ui/                                          # 通用组件/样式
│   └── types/                                           # 类型定义
```

## 与整体架构

- 数据流：`collect 拍照 → 上传图片 → 服务端识别（recognizer 4100）→ 返回候选框 → 人工结论 → 提交`
- 图片受控加载：内容接口 `GET /api/v1/images/{id}/content`（P1：小程序端 `loadImageContent` 接入中）
- 定位打卡（getLocation）与离线暂存为 P1/P2 项，见 docs/06

## 待办（docs/06 小程序节）

- P0：微信真实登录联调（AppSecret + 真机）、设备端编译联调
- P1：定位打卡、图片受控加载、离线暂存
- P2：上拉加载、推送通知、H5 同步端
