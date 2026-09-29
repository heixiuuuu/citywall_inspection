<template>
  <view class="page">
    <view v-if="loading" class="tip">加载中…</view>
    <view v-else-if="tasks.length === 0" class="tip">暂无任务</view>
    <view v-for="t in tasks" :key="t.id" class="task">
      <text class="title">{{ t.title }}</text>
      <text class="status">{{ statusText(t.status) }}</text>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getTasks } from '../../shared/api/task'

const tasks = ref([])
const loading = ref(false)

const STATUS_TEXT = {
  ASSIGNED: '待执行',
  IN_PROGRESS: '进行中',
  PENDING_REVIEW: '待审核',
  RETURNED: '退回补正',
  COMPLETED: '已完成',
}

function statusText(s) {
  return STATUS_TEXT[s] || s
}

onMounted(async () => {
  loading.value = true
  try {
    // 初始化阶段 Go 服务未实现列表接口，先按契约调用；失败静默展示空列表。
    tasks.value = (await getTasks()).data?.tasks ?? []
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.page {
  padding: 30rpx;
}
.tip {
  color: #999;
  text-align: center;
  margin-top: 80rpx;
}
.task {
  display: flex;
  justify-content: space-between;
  padding: 30rpx;
  background: #fff;
  border-radius: 12rpx;
  margin-bottom: 20rpx;
}
</style>
