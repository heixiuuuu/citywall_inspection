# miniapp/ — 巡检员现场端（uni-app Vue 3，Android App + 微信小程序双端）

> 城墙病害巡检现场端：巡检员登录、查看本人任务、逐点到场（拍照→识别→人工结论→提交），
> 支持任务外主动上报；同一套源码同时构建 **Android App（APK）** 与 **微信小程序**。
> UI/UE 设计稿见 `design/`；服务端接口见 `server/api/openapi.yaml`。

## 技术栈

- uni-app（Vue 3 语法），目标平台：App-Android（APK）+ 微信小程序
- 双工程形态：`src/` 为标准 uni-app 源码目录，**HBuilderX 可直接导入**；根目录同时提供
  `package.json`/`vite.config.js`/`index.html`，**npm CLI 也可直接构建**（两套编译共用同一份 src/）

## 页面（src/pages）

| 页面 | 功能 | 依赖接口 |
|---|---|---|
| index | 登录（开发身份/微信 code → JWT） | `/auth/login`、`/auth/wechat-login` |
| tasks | 我的巡检任务（按状态过滤） | `GET /tasks` |
| tasks/detail | 任务详情：点位列表与真实审核状态、退回补正入口 | `GET /tasks/{id}` |
| collect | 现场采集：创建记录、拍照上传（OVERVIEW/DETAIL）、识别候选查看、人工确认/误检/补录漏检、结论、提交 | `/observations`、`/images/{id}/recognitions`、`.../conclusion`、`.../submit` |
| report | 无任务主动上报（位置/照片/异常描述） | `POST /observations`（source=VOLUNTARY） |

## 运行与构建

### 依赖安装（CLI 方式，首次）

```powershell
cd miniapp
npm install
```

### 构建 Android App 资源（APK 前置资源）

```powershell
npm run build:app          # 产物 dist/build/app/（app-service.js / manifest.json / pages / static）
```

### 构建微信小程序包

```powershell
npm run build:mp-weixin    # 产物 dist/build/mp-weixin/，微信开发者工具直接导入运行
```

### HBuilderX 方式（等价）

用 HBuilderX 导入 `miniapp/`：运行到微信开发者工具，或「运行 → 运行到手机或模拟器 → 制作自定义调试基座 / 云打包」。

## APK 打包（Android 安装包）

App 资源构建通过后，产出可安装 APK 的两条路径（任选其一）：

### 方式 A：HBuilderX 云打包（推荐，无需本地 Android SDK）

1. HBuilderX 打开 `miniapp/`，登录 DCloud 账号。
2. `manifest.json` 可视化界面 → 「App 常用其他设置」→ 点击「重新获取」自动申请 **uni-app AppID**
   （形如 `__UNI__xxxxxxx`；不申请时云打包会提示，统计功能禁用但可打包）。
3. 确认「App 模块配置」勾选 Camera、Geolocation（已声明于 manifest.json `app-plus.modules`）。
4. 「App 权限配置」核对权限（相机/定位/存储/网络，已声明于 `app-plus.distribute.android.permissions`）。
5. 「Android 云端打包」→ 按需填包名（已默认 `com.citywall.inspector`）、版本号（0.2.0 / 200）、
   图标（已提供 `src/static/app-icon.png`）。
6. 云端打包产出的 APK 会写回本地 `miniapp/unpackage/release/`（或 HBuilderX 下载目录），
   安装到 Android 手机即可。

> ⚠️ **明文 HTTP**：Android 9+ 默认禁止 `http://` 明文流量。云打包界面的「使用安全网络」开关
> 在本地联调阶段请**取消勾选**（或打包后允许明文）；否则 APK 无法访问 `http://<电脑IP>:8080`。
> 正式发布应改为 `https://` 并勾选安全网络。

### 方式 B：离线打包（Android Studio）

1. 下载 DCloud 离线 SDK（版本与本项目 uni-app 编译器 5.31 匹配）。
2. 新建/复用 Android 原生工程，将 `dist/build/app/` 整体放入
   `assets/apps/__UNI__<AppID>/www/`。
3. `AndroidManifest.xml` 增加 `android:usesCleartextTraffic="true"`（联调期）与权限声明。
4. 配置签名 keystore，`gradle assembleRelease` 生成 APK。

## 真机联调（APK ↔ Go 服务）

- 电脑与手机连同一局域网；Go 服务 `listen :8080` 默认监听所有网卡，手机访问 `http://<电脑局域网IP>:8080`。
- APK 内默认后端地址为 `http://127.0.0.1:8080/api/v1`（模拟器直通电脑）；**真机必须覆盖**：
  打包前把 `src/shared/api/request.js` 的 `BASE_URL` 改为电脑 IP，或装机后写入本地存储
  `uni.setStorageSync('city_wall_api_base', 'http://<电脑IP>:8080/api/v1')`（App 端自动读取，
  小程序端忽略该覆盖逻辑）。
- 定位权限首次使用会弹授权；Go 服务保存定位精度与异常说明，供审核判断。

## 登录身份（开发环境）

服务端 `conf/server.json` 的 `auth.dev_identities` 提供开发身份映射（inspector/Insp123 等），
仅限开发环境；微信正式登录需配置 `auth.wechat_app_id / wechat_app_secret` 后走真实 code 换取 JWT，
未配置时正式通道不会静默绕过登录。

## 目录结构

```
miniapp/
├── package.json / vite.config.js / index.html   # CLI 构建（App + 小程序）
├── src/
│   ├── App.vue / main.js / manifest.json / pages.json
│   ├── pages/{index,tasks,collect,report}/      # 登录 / 我的任务 / 任务详情 / 现场采集 / 主动上报
│   ├── shared/
│   │   ├── api/                                 # 接口封装（BASE_URL 可被 App 端本地存储覆盖）
│   │   ├── auth/                                # 登录态
│   │   └── ui/                                  # 通用组件/样式
│   ├── static/app-icon.png                      # App 图标（1024×1024）
│   └── types/
└── dist/build/{app,mp-weixin}/                  # 构建产物（gitignore）
```
