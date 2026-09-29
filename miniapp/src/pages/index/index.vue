<template>
  <view class="page">
    <view class="banner">古城墙病害巡检</view>
    <button type="primary" @tap="goTasks">查看我的任务</button>
  </view>
</template>

<script setup>
function goTasks() {
  uni.navigateTo({ url: '/pages/tasks/index' })
}
</script>

<style scoped>
.page {
  padding: 40rpx 30rpx;
}
.banner {
  font-size: 40rpx;
  font-weight: 600;
  margin-bottom: 40rpx;
}
</style>
