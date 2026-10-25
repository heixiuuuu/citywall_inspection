<template>
  <view class="page">
    <view class="userbar">
      <view class="avatar">{{ initial }}</view>
      <view class="grow">
        <view class="name">{{ userName }}</view>
        <view class="faint">台州府城墙 · 病害巡检</view>
      </view>
      <button class="btn-mini" @tap="logout">退出</button>
    </view>
    <button class="btn-mini" style="margin-bottom: 24rpx" @tap="openRecords">我的巡检记录</button>

    <view class="segment">
      <view
        v-for="(tab, i) in tabs"
        :key="i"
        class="seg"
        :class="{ on: activeTab === i }"
        @tap="changeTab(i)"
      >
        {{ tab.label }}
      </view>
    </view>

    <view v-if="loading" class="empty">加载中…</view>
    <view v-else-if="!filtered.length" class="empty">暂无任务</view>

    <view v-for="t in filtered" :key="t.id" class="card" @tap="openTask(t.id)">
      <view class="row">
        <view class="grow title-sm">{{ t.title }}</view>
        <view class="tag" :class="statusClass(t.status)">{{ statusText(t.status) }}</view>
        <text class="arrow">›</text>
      </view>
      <view class="meta">
        <text>负责人：{{ assigneeLabel(t.assignee_id) }}</text>
        <text v-if="t.due_at">截止 {{ dueLabel(t.due_at) }}</text>
      </view>
    </view>

    <view class="fab" @tap="goReport">＋</view>
    <button v-if="tasks.length < total" class="btn-mini" :loading="loading" @tap="load(false)">加载更多任务（{{ tasks.length }}/{{ total }}）</button>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import { onShow, onReachBottom, onPullDownRefresh } from '@dcloudio/uni-app'
import { getTasks } from '../../shared/api/task'
import { getToken, clearToken, getUser } from '../../shared/auth/auth'
import { statusText, statusClass } from '../../shared/ui/status'

const tabs = [
  { label: '全部', value: '' },
  { label: '待执行', value: 'ASSIGNED' },
  { label: '采集中', value: 'IN_PROGRESS' },
  { label: '待审核', value: 'PENDING_REVIEW' },
  { label: '已完成', value: 'COMPLETED' },
]
const activeTab = ref(0)
const tasks = ref([])
const loading = ref(false)
const user = getUser()
const page = ref(0)
const total = ref(0)
let generation = 0

const userName = computed(() => (user && user.display_name) || '巡检员')
const initial = computed(() => (userName.value ? userName.value.slice(0, 1) : '巡'))

const filtered = computed(() => tasks.value)

onShow(() => {
  if (!getToken()) {
    uni.reLaunch({ url: '/pages/index/index' })
    return
  }
  load()
})
onReachBottom(() => { if (tasks.value.length < total.value) load(false) })
onPullDownRefresh(async () => { try { await load(true) } finally { uni.stopPullDownRefresh() } })

function changeTab(index) { activeTab.value = index; load(true) }
async function load(reset = true) {
  if (loading.value && !reset) return
  const token = reset ? ++generation : generation
  const nextPage = reset ? 1 : page.value + 1
  loading.value = true
  try {
    const response = await getTasks({ page: nextPage, page_size: 20, status: tabs[activeTab.value].value })
    if (token !== generation) return
    tasks.value = reset ? response.data.tasks || [] : [...tasks.value, ...(response.data.tasks || [])]
    page.value = nextPage
    total.value = response.data.total
  } catch { uni.showToast({ title: '加载任务失败', icon: 'none' }) }
  finally { if (token === generation) loading.value = false }
}

function openTask(id) {
  uni.navigateTo({ url: `/pages/tasks/detail?id=${id}` })
}

function goReport() {
  uni.navigateTo({ url: '/pages/report/index' })
}
function openRecords() { uni.navigateTo({ url: '/pages/records/index' }) }

function assigneeLabel(id) {
  return id === (user && user.id) ? '我' : `巡检员#${id}`
}

function dueLabel(iso) {
  return (iso || '').slice(0, 10)
}

function logout() {
  clearToken()
  uni.reLaunch({ url: '/pages/index/index' })
}
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f1e8;
  padding: 24rpx 32rpx 140rpx;
}
.userbar {
  display: flex;
  align-items: center;
  gap: 20rpx;
  margin-bottom: 24rpx;
}
.avatar {
  width: 80rpx;
  height: 80rpx;
  border-radius: 50%;
  background: #2f6f63;
  color: #fff;
  font-size: 32rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.name {
  font-size: 32rpx;
  font-weight: 600;
  color: #2b2b2b;
}
.faint {
  font-size: 22rpx;
  color: #9b9b9b;
}
.btn-mini {
  font-size: 24rpx;
  color: #2f6f63;
  background: #fff;
  border: 1rpx solid #2f6f63;
  border-radius: 12rpx;
  padding: 8rpx 24rpx;
  line-height: 1.6;
}
.segment {
  display: flex;
  background: #fff;
  border-radius: 16rpx;
  padding: 8rpx;
  margin-bottom: 24rpx;
}
.seg {
  flex: 1;
  text-align: center;
  padding: 14rpx 0;
  border-radius: 12rpx;
  font-size: 26rpx;
  color: #6b6b6b;
}
.seg.on {
  background: #2f6f63;
  color: #fff;
  font-weight: 600;
}
.card {
  background: #fff;
  border-radius: 24rpx;
  padding: 24rpx 28rpx;
  margin-bottom: 24rpx;
  box-shadow: 0 2rpx 8rpx rgba(43, 43, 43, 0.06);
}
.row {
  display: flex;
  align-items: center;
  gap: 12rpx;
}
.grow {
  flex: 1;
}
.title-sm {
  font-size: 30rpx;
  font-weight: 600;
  color: #2b2b2b;
}
.tag {
  padding: 4rpx 16rpx;
  border-radius: 8rpx;
  font-size: 22rpx;
  font-weight: 500;
}
.tag.draft { background: rgba(138, 138, 138, 0.12); color: #8a8a8a; }
.tag.assigned { background: rgba(59, 130, 196, 0.12); color: #3b82c4; }
.tag.progress { background: rgba(201, 138, 45, 0.12); color: #c98a2d; }
.tag.pending { background: rgba(122, 91, 198, 0.12); color: #7a5bc6; }
.tag.done { background: rgba(62, 155, 91, 0.12); color: #3e9b5b; }
.tag.bad { background: rgba(196, 69, 60, 0.12); color: #c4453c; }
.arrow {
  color: #9b9b9b;
  font-size: 28rpx;
}
.meta {
  display: flex;
  gap: 24rpx;
  font-size: 24rpx;
  color: #6b6b6b;
  margin-top: 12rpx;
}
.empty {
  text-align: center;
  color: #9b9b9b;
  font-size: 26rpx;
  padding: 80rpx 0;
}
.fab {
  position: fixed;
  right: 32rpx;
  bottom: 64rpx;
  width: 96rpx;
  height: 96rpx;
  border-radius: 50%;
  background: #2f6f63;
  color: #fff;
  font-size: 44rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8rpx 20rpx rgba(47, 111, 99, 0.35);
  z-index: 5;
}
</style>
