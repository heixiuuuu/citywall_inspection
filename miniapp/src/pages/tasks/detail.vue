<template>
  <view class="page">
    <view v-if="!task" class="empty">{{ statusError || '加载中…' }}<button v-if="statusError" size="mini" @tap="load">重试</button></view>

    <template v-else>
      <view class="card">
        <view class="row">
          <view class="grow title">{{ task.title }}</view>
          <view class="tag" :class="statusClass(task.status)">{{ statusText(task.status) }}</view>
        </view>
        <view class="meta">
          <text>负责人：{{ assigneeLabel }}</text>
          <text v-if="task.due_at">截止 {{ (task.due_at || '').slice(0, 10) }}</text>
        </view>
        <view v-if="task.description" class="sub">{{ task.description }}</view>
      </view>

      <view v-if="statusError" class="card">
        <view>{{ statusError }}</view>
        <button size="mini" @tap="load">重新加载</button>
      </view>
      <view class="card list">
        <view
          v-for="p in points"
          :key="p.id"
          class="point"
          @tap="openCollect(p)"
        >
          <view class="seq" :class="{ done: isDone(p) }">{{ p.sequence_no }}</view>
          <view class="grow">
            <view class="title-sm">{{ p.snapshot_name || '点位' }}</view>
            <view class="faint">{{ p.snapshot_location_hint || '—' }}</view>
          </view>
          <view class="tag" :class="pointStatusReady ? statusClass(pointStatus(p)) : ''">{{ pointStatusText(p) }}</view>
          <text class="arrow">›</text>
        </view>
      </view>
    </template>

    <view class="bottombar">
      <button class="btn-primary" :loading="submitting" :disabled="!canSubmit" @tap="submit">
        提交任务（{{ pointStatusReady ? doneCount : '—' }}/{{ points.length }} 已完成）
      </button>
      <view class="note">所有点位采集并提交后方可整任务提交</view>
    </view>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import { getTask, submitTask } from '../../shared/api/task'
import { listObservations } from '../../shared/api/observation'
import { statusText, statusClass } from '../../shared/ui/status'
import { getUser } from '../../shared/auth/auth'

const taskId = ref(null)
const task = ref(null)
const points = ref([])
const pointStatusMap = ref({})
const pointRecords = ref({})
const pointStatusReady = ref(false)
const statusLoading = ref(false)
const statusError = ref('')
const submitting = ref(false)
const user = getUser()

onLoad((query) => {
  taskId.value = Number(query.id)
})
onShow(() => { if (taskId.value) load() })

async function load() {
  if (statusLoading.value) return
  statusLoading.value = true
  pointStatusReady.value = false
  statusError.value = ''
  try {
    const res = await getTask(taskId.value)
    task.value = res.data.task
    points.value = res.data.points || []
    pointStatusMap.value = {}
    pointRecords.value = {}
    try {
      let page = 1
      let loaded = 0
      let total = 0
      do {
        const ores = await listObservations({ task_id: taskId.value, page_size: 100, page })
        const records = ores.data.items || []
        total = ores.data.total
        if (!Number.isInteger(total) || total < 0) throw new Error('invalid record total')
        for (const o of records) {
          if (o.task_point_id != null) {
            pointStatusMap.value[o.task_point_id] = o.status
            pointRecords.value[o.task_point_id] = o
          }
        }
        loaded += records.length
        page++
        if (!records.length && loaded < total) throw new Error('incomplete record list')
      } while (loaded < total)
      pointStatusReady.value = true
    } catch {
      statusError.value = '点位记录状态加载失败，请重试'
    }
  } catch { statusError.value = '任务加载失败，请重试' }
  finally { statusLoading.value = false }
}

const doneCount = computed(() => points.value.filter((p) => isDone(p)).length)
const canSubmit = computed(
  () => pointStatusReady.value && points.value.length > 0 && task.value && ['ASSIGNED', 'IN_PROGRESS'].includes(task.value.status) && doneCount.value === points.value.length
)
const assigneeLabel = computed(() => (task.value && task.value.assignee_id === (user && user.id) ? '我' : '负责人'))

function isDone(p) {
  const st = pointStatus(p)
  return st === 'SUBMITTED' || st === 'ACCEPTED'
}

function pointStatus(p) {
  return pointStatusMap.value[p.id] || 'DRAFT'
}

function pointStatusText(p) {
  return pointStatusReady.value ? statusText(pointStatus(p)) : '状态未加载'
}

function openCollect(p) {
  if (!pointStatusReady.value) { uni.showToast({ title: '请先重新加载点位状态', icon: 'none' }); return }
  const record = pointRecords.value[p.id]
  if (record && (!['DRAFT', 'RETURNED'].includes(record.status) || ['CANCELLED', 'COMPLETED'].includes(task.value.status))) {
    uni.navigateTo({ url: `/pages/records/index?observationId=${record.id}` })
    return
  }
  if (!['ASSIGNED', 'IN_PROGRESS'].includes(task.value.status) && !(task.value.status === 'PENDING_REVIEW' && pointStatus(p) === 'RETURNED')) {
    uni.showToast({ title: '任务已结束，不可采集', icon: 'none' })
    return
  }
  uni.navigateTo({
    url: record ? `/pages/collect/index?observationId=${record.id}` : `/pages/collect/index?taskId=${task.value.id}&taskPointId=${p.id}&name=${encodeURIComponent(p.snapshot_name || '点位')}`,
  })
}

function submit() {
  submitting.value = true
  submitTask(task.value.id, task.value.revision)
    .then(() => {
      uni.showToast({ title: '任务已提交审核', icon: 'success' })
      load()
    })
    .catch(() => {})
    .finally(() => {
      submitting.value = false
    })
}
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f1e8;
  padding: 24rpx 32rpx 160rpx;
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
.title {
  font-size: 34rpx;
  font-weight: 600;
  color: #2b2b2b;
}
.title-sm {
  font-size: 30rpx;
  font-weight: 600;
  color: #2b2b2b;
}
.meta {
  display: flex;
  gap: 24rpx;
  font-size: 24rpx;
  color: #6b6b6b;
  margin-top: 12rpx;
}
.sub {
  font-size: 26rpx;
  color: #6b6b6b;
  margin-top: 12rpx;
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
.list {
  padding: 8rpx 28rpx;
}
.point {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 20rpx 0;
  border-bottom: 1rpx solid #e6e1d5;
}
.point:last-child {
  border-bottom: none;
}
.seq {
  width: 56rpx;
  height: 56rpx;
  border-radius: 50%;
  border: 2rpx solid #2f6f63;
  color: #2f6f63;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26rpx;
  font-weight: 600;
  flex: none;
}
.seq.done {
  background: #2f6f63;
  color: #fff;
}
.faint {
  font-size: 22rpx;
  color: #9b9b9b;
  margin-top: 4rpx;
}
.arrow {
  color: #9b9b9b;
  font-size: 28rpx;
}
.empty {
  text-align: center;
  color: #9b9b9b;
  font-size: 26rpx;
  padding: 100rpx 0;
}
.bottombar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  background: #fff;
  border-top: 1rpx solid #e6e1d5;
  padding: 20rpx 32rpx;
  display: flex;
  flex-direction: column;
  gap: 6rpx;
}
.btn-primary {
  background: #2f6f63;
  color: #fff;
  border-radius: 16rpx;
  height: 88rpx;
  line-height: 88rpx;
  font-size: 32rpx;
  font-weight: 600;
}
.btn-primary[disabled] {
  opacity: 0.4;
  color: #fff;
}
.note {
  font-size: 22rpx;
  color: #6b6b6b;
  text-align: center;
}
</style>
