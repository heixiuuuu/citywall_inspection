<template>
  <view class="page">
    <view class="logo">
      <text class="logo-text">台州\n府城墙</text>
    </view>
    <view class="title">台州府城墙病害巡检</view>
    <view class="sub">基于视觉识别的古城墙病害巡检系统</view>

    <input v-model="username" class="input" placeholder="巡检账号" />
    <input v-model="password" class="input" password placeholder="密码" />
    <input v-model="apiBase" class="input" placeholder="服务地址，如 http://电脑IP:8080/api/v1" />
    <button class="btn btn-primary" :loading="loading" @tap="login">账号登录</button>
    <!-- #ifdef MP-WEIXIN -->
    <button class="btn" :loading="loading" @tap="loginWithWechat">微信登录</button>
    <!-- #endif -->
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { wechatLogin, accountLogin } from '../../shared/api/task'
import { getBaseUrl } from '../../shared/api/request'
import { setToken, setUser, isLoggedIn } from '../../shared/auth/auth'

const loading = ref(false)
const username = ref('')
const password = ref('')
const apiBase = ref(getBaseUrl())
onLoad(() => { if (isLoggedIn()) uni.reLaunch({ url: '/pages/tasks/index' }) })

function acceptSession(res) {
  setToken(res.data.token)
  setUser(res.data.user)
  uni.reLaunch({ url: '/pages/tasks/index' })
}

function login() {
  if (loading.value) return
  if (!username.value.trim() || !password.value) {
    uni.showToast({ title: '请输入账号和密码', icon: 'none' })
    return
  }
  if (!/^https?:\/\//i.test(apiBase.value.trim())) {
    uni.showToast({ title: '请输入完整的 http 或 https 服务地址', icon: 'none' })
    return
  }
  uni.setStorageSync('city_wall_api_base', apiBase.value.trim().replace(/\/$/, ''))
  loading.value = true
  accountLogin(username.value.trim(), password.value)
    .then(acceptSession)
    .catch(() => {
      uni.showToast({ title: '登录失败，请确认服务已启动', icon: 'none' })
    })
    .finally(() => {
      loading.value = false
    })
}

function loginWithWechat() {
  loading.value = true
  uni.login({
    provider: 'weixin',
    success: result => wechatLogin(result.code).then(acceptSession).finally(() => { loading.value = false }),
    fail: () => { loading.value = false; uni.showToast({ title: '微信登录失败', icon: 'none' }) },
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
.input { box-sizing: border-box; width: 100%; min-height: 88rpx; background: white; padding: 20rpx; margin-bottom: 20rpx; border-radius: 12rpx; }
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
