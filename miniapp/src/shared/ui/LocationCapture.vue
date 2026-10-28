<template>
  <view>
    <button class="locate" :loading="loading" :disabled="disabled || loading" @tap="locate">获取现场定位</button>
    <view class="tip">{{ label }}</view>
  </view>
</template>
<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
const props = defineProps({ value: { type: Object, default: null }, disabled: Boolean })
const emit = defineEmits(['change', 'pending'])
const loading = ref(false)
const message = ref('')
let disposed = false
onBeforeUnmount(() => { disposed = true })
const label = computed(() => loading.value ? '正在获取定位，请稍候…' : message.value || (props.value
  ? `现场位置：${props.value.latitude.toFixed(6)}, ${props.value.longitude.toFixed(6)} · 精度 ${props.value.accuracy_m == null ? '未知' : Math.round(props.value.accuracy_m) + ' m'}`
  : '尚未获取定位'))
function locate() {
  if (props.disabled || loading.value || disposed) return
  loading.value = true
  emit('pending', true)
  uni.getLocation({
    type: 'wgs84',
    isHighAccuracy: true,
    success: result => {
      if (disposed) return
      const value = { latitude: result.latitude, longitude: result.longitude, accuracy_m: result.accuracy, captured_at: new Date().toISOString() }
      message.value = ''
      emit('change', value)
    },
    fail: () => {
      if (disposed) return
      message.value = props.value ? '定位失败，保留原位置；请开启位置权限后重试' : '定位失败，请开启位置权限后重试'
      uni.showToast({ title: '无法获取定位', icon: 'none' })
    },
    complete: () => {
      if (disposed) return
      loading.value = false
      emit('pending', false)
    },
  })
}
</script>
<style scoped>
.locate { font-size: 26rpx; margin-top: 16rpx; background: #e8f0ec; color: #25584f; }
.tip { font-size: 22rpx; color: #777; margin-top: 12rpx; }
</style>
