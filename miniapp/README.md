# 巡检端

uni-app / Vue 3 工程，支持账号密码登录、任务分页查询、现场图像上传、WGS84 定位、检测候选判定、人工结论、草稿保存、退回补正及主动上报。

“我的巡检记录”按状态检索本人记录，展示原图、识别尝试、人工结论修订和审核意见。任务记录与主动上报均可从草稿或退回状态恢复编辑；已提交、通过或驳回的记录只读。

`npm ci` 安装锁定依赖；`npm run build:h5`、`npm run build:mp-weixin`、`npm run build:app` 分别构建三个平台的资源。微信登录使用 uni.login 返回的 code，后端必须配置微信凭据并绑定人员账号；账号登录不依赖微信。

服务地址可在登录页设置并保存在本机。真机使用电脑局域网地址，模拟器使用 10.0.2.2，浏览器默认 127.0.0.1；构建变量 VITE_API_BASE 可覆盖默认值。

## Android APK

android 工程使用 WebViewAssetLoader 加载 H5 文件，通过系统相机或文件选择器获取照片，使用 FileProvider 回传拍照文件，并申请定位权限。需安装 JDK 17、Android SDK 35 和 Build Tools 35.0.0，配置 JAVA_HOME、ANDROID_HOME。Gradle Wrapper 固定使用 8.13 并校验分发包哈希。

在仓库根目录执行 `./scripts/build_android.ps1`，脚本构建 H5、同步资源并生成 android/app/build/outputs/apk/debug/app-debug.apk。H5 已完成构建时可传入 -SkipWebBuild。

安装到设备后，验证登录、系统相机拍照回传、图片上传、定位授权与拒绝、任务草稿恢复和后台恢复。模拟器可验证软件集成，真实定位精度与现场相机效果需用实机核验。

App 资源构建成功不能代替 APK 构建与真机验收。正式签名通过本地 Gradle 属性配置，不将签名文件与密码提交到仓库。
