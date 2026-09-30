<template>
  <view class="page">
    <view class="logo">
      <text class="logo-text">台州\n府城墙</text>
    </view>
    <view class="title">台州府城墙病害巡检</view>
    <view class="sub">基于视觉识别的古城墙病害巡检系统</view>

    <button class="btn btn-primary" :loading="loading" @tap="login">微信一键登录</button>
    <view class="faint">MVP 测试身份：test-inspector / test-reviewer / test-admin</view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { wechatLogin } from '../../shared/api/task'
import { setToken, setUser } from '../../shared/auth/auth'

const loading = ref(false)

function login() {
  loading.value = true
  wechatLogin('test-inspector')
    .then((res) => {
      setToken(res.data.token)
      setUser(res.data.user)
      uni.reLaunch({ url: '/pages/tasks/index' })
    })
    .catch(() => {
      uni.showToast({ title: '登录失败，请确认服务已启动', icon: 'none' })
    })
    .finally(() => {
      loading.value = false
    })
}
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f1e8;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0 48rpx;
}
.logo {
  width: 176rpx;
  height: 176rpx;
  border-radius: 48rpx;
  background: #25584f;
  color: #fff;
  font-size: 30rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  margin-bottom: 40rpx;
}
.logo-text {
  line-height: 1.35;
}
.title {
  font-size: 36rpx;
  font-weight: 600;
  color: #2b2b2b;
  margin-bottom: 12rpx;
}
.sub {
  font-size: 26rpx;
  color: #6b6b6b;
  margin-bottom: 80rpx;
}
.btn-primary {
  width: 100%;
  margin-bottom: 24rpx;
  background: #2f6f63;
  color: #fff;
  border-radius: 16rpx;
  height: 88rpx;
  line-height: 88rpx;
  font-size: 32rpx;
  font-weight: 600;
}
.faint {
  font-size: 22rpx;
  color: #9b9b9b;
}
</style>
