# 巡检端

uni-app / Vue 3 工程，支持账号密码登录、任务分页查询、现场图像上传、WGS84 定位、检测候选判定、人工结论、草稿保存、退回补正及主动上报。

“我的巡检记录”按状态检索本人记录，展示原图、识别尝试、人工结论修订和审核意见。任务记录与主动上报均可从草稿或退回状态恢复编辑；已提交、通过或驳回的记录只读。

`npm ci` 安装锁定依赖；`npm run build:h5`、`npm run build:mp-weixin`、`npm run build:app` 分别构建三个平台的资源。微信登录使用 uni.login 返回的 code，后端必须配置微信凭据并绑定人员账号；账号登录不依赖微信。

服务地址可在登录页设置并保存在本机。真机使用电脑局域网地址，模拟器使用 10.0.2.2，浏览器默认 127.0.0.1；构建变量 VITE_API_BASE 可覆盖默认值。

## Android APK

android 工程使用 WebViewAssetLoader 加载 H5 文件，支持文件选择和定位权限申请。需安装 JDK 17、Android SDK 35 及与 Android Gradle Plugin 8.13.2 兼容的 Gradle，并配置 JAVA_HOME、ANDROID_HOME。

1. 在 miniapp 执行 `npm run build:h5`。
2. 将 dist/build/h5 的内容同步至 android/app/src/main/assets/www。
3. 在 android 执行 `gradle assembleDebug`。
4. 将 app/build/outputs/apk/debug/app-debug.apk 安装到设备，验证相机、定位、网络及后台恢复。

App 资源构建成功不能代替 APK 构建与真机验收。正式签名通过本地 Gradle 属性配置，不将签名文件与密码提交到仓库。
