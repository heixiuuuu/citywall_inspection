<template>
  <view>
    <button class="locate" :loading="loading" @tap="locate">获取现场定位</button>
    <view class="tip">{{ message }}</view>
  </view>
</template>
<script setup>
import { ref } from 'vue'
const emit = defineEmits(['change'])
const loading = ref(false)
const message = ref('尚未获取定位')
function locate() {
  loading.value = true
  uni.getLocation({
    type: 'wgs84',
    isHighAccuracy: true,
    success: result => {
      const value = { latitude: result.latitude, longitude: result.longitude, accuracy_m: result.accuracy, captured_at: new Date().toISOString() }
      message.value = `${value.latitude.toFixed(6)}, ${value.longitude.toFixed(6)} · 精度 ${Math.round(value.accuracy_m)} m`
      emit('change', value)
    },
    fail: () => { message.value = '定位失败，请开启位置权限后重试'; uni.showToast({ title: '无法获取定位', icon: 'none' }) },
    complete: () => { loading.value = false },
  })
}
</script>
<style scoped>
.locate { font-size: 26rpx; margin-top: 16rpx; background: #e8f0ec; color: #25584f; }
.tip { font-size: 22rpx; color: #777; margin-top: 12rpx; }
</style>
