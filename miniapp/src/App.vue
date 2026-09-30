<script>
import { isLoggedIn } from './shared/auth/auth'

export default {
  onLaunch() {
    // 启动时校验登录态：无 token 一律回登录页（路由守卫在页面 onShow 内兜底）。
    if (!isLoggedIn()) {
      uni.reLaunch({ url: '/pages/index/index' })
    }
  },
}
</script>

<style>
/* 全局设计 token（与 design/miniapp/ui-spec.md 对齐） */
page {
  background-color: #f5f1e8;
  font-size: 28rpx;
  color: #2b2b2b;
  font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
}
button::after {
  border: none;
}
</style>
