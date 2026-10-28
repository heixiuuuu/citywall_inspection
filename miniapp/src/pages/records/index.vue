<template>
  <view class="page">
    <template v-if="detailId">
      <button class="back" @tap="backToList">返回我的记录</button>
      <view v-if="loading" class="empty">正在加载记录…</view>
      <template v-else-if="evidence">
        <view class="card">
          <view class="row"><text class="title">记录 #{{ evidence.observation.id }}</text><text class="status">当前{{ statusText(evidence.observation.status) }}</text></view>
          <picker v-if="evidence.submissions?.length" :value="versionIndex" :range="versionLabels" @change="changeVersion"><view class="version">{{ versionLabels[versionIndex] }} · 切换</view></picker>
          <view v-if="snapshot" class="muted">提交快照只读，保留本次提交的位置、照片和人工结论。</view>
          <view class="line">{{ evidence.observation.source === 'TASK' ? '任务巡检' : '主动上报' }} · {{ shownLocation.location_text }}</view>
          <view class="muted">提交 {{ formatTime(snapshot?.submitted_at || evidence.observation.submitted_at) }}（北京时间）</view>
          <view class="muted" v-if="shownLocation.latitude != null">位置 {{ shownLocation.latitude }}, {{ shownLocation.longitude }} · 精度 {{ shownLocation.accuracy_m ?? '—' }} m</view>
          <view v-if="shownLocation.distance_m != null" class="muted">距任务目标 {{ shownLocation.distance_m.toFixed(1) }} m · 提示半径 {{ locationRadius ?? '—' }} m</view>
          <view v-if="locationWarning" class="location-warning">{{ locationWarning }}</view>
          <button v-if="canEdit" class="primary" @tap="continueEditing">{{ evidence.observation.status === 'RETURNED' ? '补正并重新提交' : '继续编辑草稿' }}</button>
        </view>
        <view v-if="evidence.observation.admitted_at" class="card">
          <view class="title">当前受理结果</view>
          <view class="line">{{ evidence.observation.no_issue_reason ? `不立项：${evidence.observation.no_issue_reason}` : '已立项或关联问题' }}</view>
          <view class="muted">{{ formatTime(evidence.observation.admitted_at) }}</view>
        </view>
        <view class="card" v-if="shownReviews.length">
          <view class="title">审核意见</view>
          <view v-for="review in shownReviews" :key="review.id" class="item">
            <view>{{ decisionText(review.decision) }}</view>
            <view v-if="review.disposition">处置意见：{{ review.disposition }}</view>
            <view v-if="review.reason">审核原因：{{ review.reason }}</view>
            <view class="muted">{{ formatTime(review.created_at) }}</view>
          </view>
        </view>
        <view class="card">
          <view class="title">现场照片</view>
          <view v-if="!images.length" class="muted">暂无照片</view>
          <view v-for="image in images" :key="image.imageId" class="item">
            <DetectionImage v-if="image.localPath" :src="image.localPath" :boxes="latestDetections(image.imageId)" @preview="preview" />
            <view v-else class="muted">图片加载失败</view>
            <view class="muted">{{ image.role === 'OVERVIEW' ? '概览' : image.role === 'DETAIL' ? '细节' : '历史照片' }} #{{ image.imageId }}</view>
          </view>
        </view>
        <view class="card">
          <view class="title">识别结果</view>
          <view class="muted">候选仅供人工复核，未检出不代表正常。</view>
          <view v-if="!shownAttempts.length" class="item">{{ snapshot ? '提交时无已完成识别' : '尚未执行识别' }}</view>
          <view v-for="attempt in shownAttempts" :key="attempt.id" class="item">
            <view>图片 #{{ attempt.image_id }} · 第 {{ attempt.attempt_no }} 次 · {{ attempt.status === 'SUCCEEDED' ? '识别成功' : attempt.status === 'FAILED' ? '识别失败' : '识别中' }}</view>
            <view class="muted">{{ attempt.model_version || attempt.error_code || '—' }}</view>
            <view v-for="(detection, index) in evidence.detections.filter(item => item.attempt_id === attempt.id)" :key="detection.id" class="candidate">
              {{ index + 1 }} · {{ categoryLabel(detection.class_code) }} · {{ (detection.confidence * 100).toFixed(1) }}%<view class="muted">原图坐标 ({{ detection.x1 }}, {{ detection.y1 }})—({{ detection.x2 }}, {{ detection.y2 }})</view>
            </view>
            <view v-if="analysis(attempt)?.status === 'SUCCEEDED'" class="analysis">
              <view>{{ analysis(attempt).description }}</view>
              <view>可能成因：{{ analysis(attempt).possible_causes.join('；') || '无法判断' }}</view>
              <view>复查建议：{{ analysis(attempt).recommendations.join('；') || '需人工复查' }}</view>
              <view class="muted">辅助分析须经人工复核。</view>
            </view>
          </view>
        </view>
        <view class="card">
          <view class="title">{{ snapshot ? '本次提交人工结论' : '人工结论与修订' }}</view>
          <view v-if="!shownConclusions.length" class="muted">尚未填写人工结论</view>
          <view v-for="conclusion in shownConclusions" :key="conclusion.id" class="item">
            <view>第 {{ conclusion.revision_no }} 版 · {{ outcomeText(conclusion.outcome) }}</view>
            <view class="line">{{ conclusion.description || '无补充说明' }}</view>
            <view v-for="item in evidence.items.filter(row => row.conclusion_revision_id === conclusion.id)" :key="item.id" class="muted">图片 #{{ item.image_id }} · {{ decisionText(item.decision) }} · {{ categoryLabel(item.final_class_code) }} {{ item.description }}</view>
          </view>
        </view>
      </template>
      <view v-else class="empty">记录无法加载<button @tap="openDetail(detailId)">重试</button></view>
    </template>
    <template v-else>
      <view class="filters"><view v-for="filter in filters" :key="filter.value" :class="['filter', {active: status === filter.value}]" @tap="changeStatus(filter.value)">{{ filter.label }}</view></view>
      <view v-for="record in records" :key="record.id" class="card record" @tap="openDetail(record.id)">
        <view class="row"><text class="title">{{ record.location_text || '未填写位置' }}</text><text class="status">{{ statusText(record.status) }}</text></view>
        <view class="muted">{{ record.source === 'TASK' ? '任务巡检' : '主动上报' }} · #{{ record.id }} · {{ formatTime(record.submitted_at || record.created_at) }}</view>
      </view>
      <view v-if="!records.length && !loading" class="empty">暂无记录</view>
      <button v-if="hasMore" :loading="loading" @tap="loadList(false)">加载更多</button>
      <view v-else class="muted ending">{{ loading ? '正在加载…' : '已显示全部记录' }}</view>
    </template>
  </view>
</template>
<script setup>
import { computed, ref } from 'vue'
import { onLoad, onShow, onReachBottom, onUnload } from '@dcloudio/uni-app'
import { listObservations, loadObservation } from '../../shared/api/observation'
import { statusText } from '../../shared/ui/status'
import { formatTime } from '../../shared/ui/time'
import DetectionImage from '../../shared/ui/DetectionImage.vue'
import { categoryLabel } from '../../shared/ui/categories'
import { getUser } from '../../shared/auth/auth'
import { loadImageContent } from '../../shared/api/request'
const filters = [{value: '', label: '全部'}, {value: 'DRAFT', label: '草稿'}, {value: 'SUBMITTED', label: '待审核'}, {value: 'RETURNED', label: '已退回'}, {value: 'ACCEPTED', label: '已通过'}, {value: 'REJECTED', label: '已驳回'}]
const records = ref([])
const status = ref('')
const page = ref(0)
const total = ref(0)
const loading = ref(false)
const detailId = ref(0)
const evidence = ref(null)
const images = ref([])
const currentMetas = ref([])
const versionIndex = ref(0)
const snapshot = computed(() => evidence.value?.submissions?.[versionIndex.value - 1] || null)
const versionLabels = computed(() => ['当前记录', ...(evidence.value?.submissions || []).map(submission => `第 ${submission.submission_no} 次提交 · ${formatTime(submission.submitted_at)}`)])
const shownLocation = computed(() => snapshot.value?.location || evidence.value?.observation || {})
const locationRadius = computed(() => snapshot.value?.location.radius_m ?? evidence.value?.location_radius_m)
const locationWarning = computed(() => [
  shownLocation.value.distance_m != null && locationRadius.value != null && shownLocation.value.distance_m > locationRadius.value ? '超出任务点提示半径，请结合现场情况复核' : '',
  shownLocation.value.accuracy_m != null && shownLocation.value.accuracy_m > 30 ? '定位精度较低，请复核位置' : '',
].filter(Boolean).join('；'))
const shownReviews = computed(() => (evidence.value?.reviews || []).filter(review => !snapshot.value || review.submission_id === snapshot.value.id))
const shownConclusions = computed(() => (evidence.value?.conclusions || []).filter(conclusion => !snapshot.value || conclusion.id === snapshot.value.conclusion_revision_id))
const shownAttempts = computed(() => (evidence.value?.attempts || []).filter(attempt => images.value.some(image => image.imageId === attempt.image_id) && (!snapshot.value || attempt.finished_at && Date.parse(attempt.finished_at) <= Date.parse(snapshot.value.submitted_at))))
let generation = 0
const hasMore = computed(() => records.value.length < total.value)
const canEdit = computed(() => !snapshot.value && evidence.value && evidence.value.observation.reporter_id === getUser()?.id && ['DRAFT', 'RETURNED'].includes(evidence.value.observation.status))
onLoad(query => { if (query.observationId) detailId.value = Number(query.observationId) })
onShow(() => { if (detailId.value) openDetail(detailId.value); else loadList(true) })
onReachBottom(() => { if (!detailId.value && hasMore.value) loadList(false) })
onUnload(() => { generation++; releaseImages() })
function revokeImages(values) {
  // #ifdef H5
  values.forEach(image => { if (image.localPath.startsWith('blob:')) URL.revokeObjectURL(image.localPath) })
  // #endif
}
function releaseImages() {
  revokeImages(images.value)
  images.value = []
}
async function loadList(reset) {
  if (loading.value && !reset) return
  const token = reset ? ++generation : generation
  const next = reset ? 1 : page.value + 1
  loading.value = true
  try {
    const response = await listObservations({ status: status.value, page: next, page_size: 20 })
    if (token !== generation) return
    records.value = reset ? response.data.items || [] : [...records.value, ...(response.data.items || [])]
    total.value = response.data.total
    page.value = next
  } catch { uni.showToast({ title: '加载记录失败', icon: 'none' }) }
  finally { if (token === generation) loading.value = false }
}
function changeStatus(value) { status.value = value; loadList(true) }
async function openDetail(id) {
  const token = ++generation
  detailId.value = id
  evidence.value = null
  versionIndex.value = 0
  currentMetas.value = []
  releaseImages()
  loading.value = true
  try {
    const loaded = await loadObservation(id)
    if (token !== generation) { revokeImages(loaded.images); return }
    evidence.value = loaded.evidence
    currentMetas.value = loaded.images.map(image => ({ imageId: image.imageId, role: image.role }))
    images.value = loaded.images
  } catch { uni.showToast({ title: '加载详情失败', icon: 'none' }) }
  finally { if (token === generation) loading.value = false }
}
async function changeVersion(event) {
  versionIndex.value = Number(event.detail.value)
  const token = ++generation
  const metas = snapshot.value ? snapshot.value.images.map(image => ({ imageId: image.id, role: image.role })) : currentMetas.value
  releaseImages()
  loading.value = true
  try {
    const loaded = await Promise.all(metas.map(async image => ({ ...image, localPath: await loadImageContent(image.imageId).catch(() => '') })))
    if (token !== generation) { revokeImages(loaded); return }
    images.value = loaded
  } finally { if (token === generation) loading.value = false }
}
function backToList() { detailId.value = 0; evidence.value = null; releaseImages(); loadList(true) }
function continueEditing() {
  const record = evidence.value.observation
  const target = record.source === 'TASK' ? 'collect' : 'report'
  uni.navigateTo({ url: `/pages/${target}/index?observationId=${record.id}` })
}
function preview(current) { uni.previewImage({ current, urls: images.value.map(image => image.localPath).filter(Boolean) }) }
function latestDetections(imageId) {
  const attempts = shownAttempts.value.filter(item => item.image_id === imageId)
  const latest = attempts[attempts.length - 1]
  return evidence.value?.detections.filter(item => item.attempt_id === latest?.id) || []
}
function analysis(attempt) { try { return JSON.parse(attempt.analysis_json || 'null') } catch { return null } }
function outcomeText(value) { return { NORMAL: '正常', SUSPECTED: '疑似病害', UNDETERMINED: '不确定', UNREACHABLE: '无法到达' }[value] || value }
function decisionText(value) { return { ACCEPT: '审核通过', RETURN: '退回补正', REJECT: '拒绝', CONFIRM: '确认', MODIFY: '修改', ADD: '补录' }[value] || value }
</script>
<style scoped>
.location-warning { margin-top: 12rpx; color: #9f422d; font-size: 26rpx; line-height: 1.5; }
.page { min-height: 100vh; padding: 24rpx 28rpx 60rpx; background: #f5f1e8; box-sizing: border-box; color: #2b2b2b; }
.card { padding: 28rpx; margin-bottom: 24rpx; background: white; border-radius: 20rpx; }
.row { display: flex; justify-content: space-between; align-items: center; gap: 20rpx; }
.title { font-size: 30rpx; font-weight: 600; }
.status { flex-shrink: 0; color: #2f6f63; font-size: 24rpx; }
.line { margin-top: 16rpx; font-size: 27rpx; white-space: pre-wrap; }
.muted { color: #777; font-size: 23rpx; line-height: 1.6; margin-top: 10rpx; overflow-wrap: anywhere; }
.item { margin-top: 24rpx; font-size: 27rpx; line-height: 1.6; }
.photo { display: block; width: 100%; border-radius: 12rpx; }
.filters { display: flex; flex-wrap: wrap; gap: 12rpx; margin-bottom: 24rpx; }
.filter { padding: 12rpx 20rpx; background: white; border-radius: 12rpx; font-size: 25rpx; }
.filter.active, .primary { color: white; background: #2f6f63; }
.primary, .back { margin: 20rpx 0; font-size: 27rpx; }
.version { margin-top: 18rpx; padding: 16rpx; background: #f1f5f2; border-radius: 12rpx; font-size: 25rpx; color: #2f6f63; }
.empty, .ending { text-align: center; padding: 60rpx 0; color: #777; }
.candidate, .analysis { margin-top: 16rpx; padding: 16rpx; background: #f1f5f2; border-radius: 12rpx; }
</style>
