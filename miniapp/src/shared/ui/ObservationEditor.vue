<template>
  <view class="page">
    <view v-if="returnReason" class="return-reason">退回原因：{{ returnReason }}</view>
    <view class="field">
      <view class="field-label">
        现场照片 <text class="req">概览图至少 1 张</text>
      </view>
      <view class="img-grid">
        <view
          v-for="img in images"
          :key="img.localId"
          class="img-cell"
          :class="{ selected: selectedImageId === img.imageId }"
          @tap="selectedImageId = img.imageId"
        >
          <image :src="img.localPath" mode="aspectFill" class="img" />
          <view v-if="!busy" class="del" @tap.stop="removeImage(img)">×</view>
          <view class="tag-ov" :class="{ detail: img.role === 'DETAIL' }">
            {{ img.role === 'OVERVIEW' ? '概览' : '细节' }}
          </view>
        </view>
        <template v-if="!busy"><view v-for="n in slots" :key="'add' + n" class="img-cell add" @tap="chooseImage">＋</view></template>
      </view>
      <view v-if="photoBusy" class="faint">正在处理照片，请稍候…</view>
      <view class="faint">拍照或从相册选择 · 首张为概览图，后续为细节图</view>
    </view>

    <view class="field">
      <view class="field-label">位置 <text class="req">必填</text></view>
      <view class="loc-box">
        <view class="loc-line"><view class="loc-dot" />位置描述：{{ locationText || '待填写' }}</view>
        <input class="input" v-model="locationText" placeholder="填写具体位置描述" />
        <LocationCapture :value="location" :disabled="busy" @pending="locationBusy = $event" @change="location = $event" />
      </view>
    </view>

    <view class="field">
      <view class="field-label">现场判断 <text class="req">必选</text></view>
      <view class="pill-row">
        <view
          v-for="o in outcomes"
          :key="o.value"
          class="pill"
          :class="{ on: overall === o.value }"
          @tap="overall = o.value"
        >
          {{ o.label }}
        </view>
      </view>
    </view>

    <view class="field" v-if="selectedImage">
      <view class="field-label">
        视觉识别 <text class="faint">候选仅作参考，不替代人工结论；剥落候选可能误检或漏检</text>
      </view>
      <DetectionImage v-if="selectedImage.localPath" :src="selectedImage.localPath" :boxes="detections" :manual-boxes="manualBoxes" :drawing="drawing" @draw="addBoxedConclusion" @preview="previewImage" />
      <view v-else class="detect-note">照片预览加载失败，请返回记录后重试。</view>
      <view class="detect-note">
        {{ selectedImage.role === 'OVERVIEW' ? '概览' : '细节' }} #{{ selectedImageId }} · {{ attempt ? (attempt.status === 'SUCCEEDED' ? `候选 ${detections.length} 个` : attempt.status === 'FAILED' ? '识别失败' : '识别中') : '尚未识别' }}
        <view v-if="attempt">{{ attempt.model_version || attempt.error_code }} · 未检出不代表正常</view>
      </view>
      <button class="btn-ghost" :disabled="busy" :loading="recognizing" @tap="runRecognition">{{ recognizing ? '正在识别' : '识别当前照片' }}</button>
      <button class="btn-ghost" :disabled="busy || !selectedImage.localPath" @tap="drawing = !drawing">{{ drawing ? '取消框选' : '框选补录病害' }}</button>
      <view v-if="drawing" class="faint">在照片上拖动框选病害位置</view>
      <view v-if="visionAnalysis?.status === 'SUCCEEDED'" class="detect-note analysis">
        <view>{{ visionAnalysis.description }}</view>
        <view>可能成因：{{ visionAnalysis.possible_causes.join('；') || '无法判断' }}</view>
        <view>复查建议：{{ visionAnalysis.recommendations.join('；') || '需人工复查' }}</view>
        <view class="faint">辅助分析须经人工复核。</view>
      </view>
      <view class="detect-list">
        <view v-for="(d, index) in detections" :key="d.id" class="detect-item">
          <view class="chip brick">{{ index + 1 }} · {{ categoryLabel(d.class_code) }} {{ (d.confidence * 100).toFixed(1) }}%</view>
          <view class="row">
            <button size="mini" @tap="useCandidate(d, 'CONFIRM')">确认</button>
            <button size="mini" @tap="useCandidate(d, 'MODIFY')">修改类别</button>
            <button size="mini" @tap="useCandidate(d, 'REJECT')">误检</button>
          </view>
        </view>
      </view>
    </view>

    <view class="field">
      <view class="field-label">人工结论 <text class="req">必填</text></view>
      <view class="card">
        <view v-if="!conclusionItems.length" class="empty-sm">尚未填写结论项</view>
        <view v-for="(it, i) in conclusionItems" :key="i" class="concl-item">
          <view class="thumb"><image v-if="it.thumb" :src="it.thumb" mode="aspectFill" class="img" /></view>
          <view class="grow">
            <view class="row">
              <view class="chip" :class="'c-' + it.decision.toLowerCase()">{{ decisionText(it.decision) }}</view>
              <CategoryPicker v-model="it.final_class_code" :disabled="it.decision === 'REJECT'" @update:model-value="changeCategory(it)" />
            </view>
            <input class="input-sm" v-model="it.description" placeholder="补充描述（可选）" />
          </view>
          <view class="del" @tap="removeConclusion(i)">×</view>
        </view>
        <button class="btn-ghost" @tap="addConclusion">＋ 添加结论项</button>
      </view>
    </view>

    <view class="field">
      <view class="field-label">补充说明</view>
      <view v-if="voluntary" class="description-label">现场描述 <text class="req">必填</text></view>
      <textarea class="input" v-model="overallNote" :placeholder="voluntary ? '描述发现的病害或异常情况' : '补充说明（可选）'" />
    </view>

    <view class="bottombar">
      <button class="btn-ghost" :disabled="busy" :loading="submitting" @tap="saveDraft">保存草稿</button>
      <button class="btn-primary" :disabled="busy" :loading="submitting" @tap="submit">{{ voluntary ? '提交上报' : '提交记录' }}</button>
      <view class="note">{{ voluntary ? '主动上报将由审核员在管理端审核，可驳回' : '保存草稿后可继续补充 · 提交后进入待审核' }}</view>
    </view>
  </view>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import LocationCapture from './LocationCapture.vue'
import CategoryPicker from './CategoryPicker.vue'
import DetectionImage from './DetectionImage.vue'
import { categoryLabel } from './categories'
import { loadImageContent } from '../api/request'
import {
  createObservation,
  addImage,
  updateObservation,
  recognize,
  saveConclusion,
  submitObservation,
  listObservations,
  removeImage as deleteImage,
  loadObservation,
} from '../api/observation'

const props = defineProps({ source: { type: String, required: true }, query: { type: Object, required: true } })
const voluntary = computed(() => props.source === 'VOLUNTARY')

const outcomes = [
  { value: 'NORMAL', label: '正常' },
  { value: 'SUSPECTED', label: '疑似病害' },
  { value: 'UNDETERMINED', label: '不确定' },
  { value: 'UNREACHABLE', label: '不可达' },
]

const taskPointId = ref(null)
const obs = ref(null)
const images = ref([])
const locationText = ref('')
const location = ref(null)
const overall = ref('UNDETERMINED')
const overallNote = ref('')
const conclusionItems = ref([])
const selectedImageId = ref(null)
const drawing = ref(false)
const recognitionByImage = ref({})
const selectedImage = computed(() => images.value.find(image => image.imageId === selectedImageId.value))
const attempt = computed(() => recognitionByImage.value[selectedImageId.value]?.attempt)
const detections = computed(() => recognitionByImage.value[selectedImageId.value]?.detections || [])
const manualBoxes = computed(() => conclusionItems.value.filter(item => item.imageId === selectedImageId.value && item.decision === 'ADD' && item.final_box_json).flatMap(item => {
  try { return [JSON.parse(item.final_box_json)] } catch { return [] }
}))
const visionAnalysis = computed(() => { try { return JSON.parse(attempt.value?.analysis_json || 'null') } catch { return null } })
const recognizing = ref(false)
const submitting = ref(false)
const photoBusy = ref(false)
const locationBusy = ref(false)
const busy = computed(() => recognizing.value || submitting.value || photoBusy.value || locationBusy.value)
const returnReason = ref('')
let disposed = false

function useCandidate(detection, decision) {
  const image = images.value.find(i => i.imageId === selectedImageId.value)
  const existing = conclusionItems.value.findIndex(i => i.source_detection_id === detection.id)
  const item = {
    imageId: selectedImageId.value, thumb: image?.localPath || '',
    source_detection_id: detection.id, decision, final_class_code: detection.class_code,
    final_box_json: JSON.stringify({ x1: detection.x1, y1: detection.y1, x2: detection.x2, y2: detection.y2 }),
    description: '',
  }
  if (existing >= 0) conclusionItems.value[existing] = item
  else conclusionItems.value.push(item)
}

function changeCategory(item) {
  if (item.source_detection_id && item.decision === 'CONFIRM') item.decision = 'MODIFY'
}

onMounted(() => {
  const query = props.query
  taskPointId.value = Number(query.taskPointId)
  locationText.value = voluntary.value ? '' : decodeURIComponent(query.name || '点位')
  if (query.observationId) loadExisting(Number(query.observationId))
  else boot()
})

const slots = computed(() => Math.max(0, 6 - images.value.length))

async function boot() {
  try {
    const res = await createObservation(props.source, voluntary.value ? null : taskPointId.value, locationText.value || '未填写')
    obs.value = res.data.observation
  } catch (e) {
    if (!voluntary.value && e && e.statusCode === 409) {
      // 点位已有记录：加载已有草稿/退回记录，退回补正时允许继续提交。
      await resumeExisting()
    } else {
      uni.showToast({ title: '创建记录失败', icon: 'none' })
    }
  }
}

async function resumeExisting() {
  try {
    const response = await listObservations({ task_point_id: taskPointId.value, page_size: 1 })
    const record = (response.data.items || [])[0]
    if (!record) throw new Error('record missing')
    await loadExisting(record.id)
  } catch {
    uni.showToast({ title: '加载已有记录失败', icon: 'none' })
  }
}

async function loadExisting(id) {
  try {
    const loaded = await loadObservation(id)
    if (disposed) { loaded.images.forEach(image => releaseImage(image.localPath)); return }
    const evidence = loaded.evidence
    const record = evidence.observation
    if (record.source !== props.source || !['DRAFT', 'RETURNED'].includes(record.status) || ['CANCELLED', 'COMPLETED'].includes(evidence.task_status)) {
      loaded.images.forEach(image => releaseImage(image.localPath))
      uni.redirectTo({ url: `/pages/records/index?observationId=${id}` })
      return
    }
    obs.value = record
    images.value = loaded.images
    locationText.value = record.location_text
    overall.value = record.inspection_outcome || 'UNDETERMINED'
    if (record.latitude != null && record.longitude != null) {
      location.value = { latitude: record.latitude, longitude: record.longitude, accuracy_m: record.accuracy_m, captured_at: record.captured_at }
    }
    const latest = evidence.conclusions[evidence.conclusions.length - 1]
    if (latest) {
      overall.value = latest.outcome
      overallNote.value = latest.description
      conclusionItems.value = evidence.items
        .filter(item => item.conclusion_revision_id === latest.id && images.value.some(image => image.imageId === item.image_id))
        .map(item => ({ ...item, imageId: item.image_id, thumb: images.value.find(image => image.imageId === item.image_id)?.localPath || '' }))
    }
    for (const attempt of evidence.attempts) {
      recognitionByImage.value[attempt.image_id] = { attempt, detections: evidence.detections.filter(item => item.attempt_id === attempt.id) }
    }
    selectedImageId.value = images.value[0]?.imageId || null
    if (record.status === 'RETURNED') returnReason.value = evidence.reviews[evidence.reviews.length - 1]?.reason || '请补充现场信息'
  } catch {
    uni.showToast({ title: '加载已有记录失败', icon: 'none' })
  }
}

function chooseImage() {
  if (busy.value || disposed || images.value.length >= 6) return
  if (!obs.value) { uni.showToast({ title: '记录尚未创建', icon: 'none' }); return }
  photoBusy.value = true
  uni.chooseImage({
    count: 1,
    sizeType: ['compressed'],
    sourceType: ['camera', 'album'],
    success: async (res) => {
      const localPath = res.tempFilePaths[0]
      const role = images.value.some(image => image.role === 'OVERVIEW') ? 'DETAIL' : 'OVERVIEW'
      let imageId
      try {
        const result = await addImage(obs.value.id, localPath, role)
        const image = result.data.image
        const previewPath = await loadImageContent(image.id).catch(() => '')
        if (disposed) { releaseImage(previewPath); return }
        images.value.push({ localId: `img_${image.id}`, localPath: previewPath, imageId: image.id, role })
        selectedImageId.value = image.id
        imageId = image.id
        uni.showToast({ title: '上传成功', icon: 'success' })
      } catch {
        if (!disposed) uni.showToast({ title: '上传失败', icon: 'none' })
      } finally { photoBusy.value = false }
      if (imageId && role === 'OVERVIEW') runRecognitionOn(imageId)
    },
    fail: () => { photoBusy.value = false },
  })
}

async function removeImage(image) {
  if (busy.value || disposed || !images.value.some(item => item.imageId === image.imageId)) return
  photoBusy.value = true
  try {
    await deleteImage(obs.value.id, image.imageId)
    images.value = images.value.filter(item => item.imageId !== image.imageId)
    conclusionItems.value = conclusionItems.value.filter(item => item.imageId !== image.imageId)
    delete recognitionByImage.value[image.imageId]
    if (selectedImageId.value === image.imageId) selectedImageId.value = images.value[0]?.imageId || null
    releaseImage(image.localPath)
  } catch { if (!disposed) uni.showToast({ title: '照片移除失败', icon: 'none' }) }
  finally { photoBusy.value = false }
}

async function runRecognition() {
  if (!selectedImageId.value) {
    uni.showToast({ title: '请先选择照片', icon: 'none' })
    return
  }
  runRecognitionOn(selectedImageId.value)
}

async function runRecognitionOn(imageId) {
  if (busy.value || disposed) return
  recognizing.value = true
  try {
    const res = await recognize(imageId)
    const attempt = res.data.attempt
    const dets = res.data.detections || []
    if (disposed || !images.value.some(image => image.imageId === imageId)) return
    recognitionByImage.value[imageId] = { attempt, detections: dets }
    if (attempt.status === 'FAILED') {
      uni.showToast({ title: `识别失败：${attempt.error_code || '未知错误'}`, icon: 'none' })
    } else if (!dets.length) {
      uni.showToast({ title: '未检出候选', icon: 'none' })
    }
  } catch (e) {
    if (!disposed) uni.showToast({ title: '识别服务不可用', icon: 'none' })
  } finally {
    recognizing.value = false
  }
}

function decisionText(d) {
  return { CONFIRM: '确认', MODIFY: '修改', REJECT: '误检', ADD: '补录' }[d] || d
}

function addConclusion() {
  const firstImg = selectedImage.value || images.value[0]
  if (!firstImg) { uni.showToast({ title: '请先添加现场照片', icon: 'none' }); return }
  conclusionItems.value.push({
    imageId: firstImg ? firstImg.imageId : null,
    thumb: firstImg ? firstImg.localPath : '',
    decision: 'ADD',
    final_class_code: '',
    description: '',
  })
}

function addBoxedConclusion(box) {
  const image = selectedImage.value
  if (!image) return
  conclusionItems.value.push({ imageId: image.imageId, thumb: image.localPath, decision: 'ADD', final_class_code: '', description: '', final_box_json: JSON.stringify(box) })
  drawing.value = false
  uni.showToast({ title: '已框选，请选择病害类别', icon: 'none' })
}

function previewImage(current) { uni.previewImage({ current, urls: images.value.map(image => image.localPath).filter(Boolean) }) }
function releaseImage(path) {
  // #ifdef H5
  if (path?.startsWith('blob:')) URL.revokeObjectURL(path)
  // #endif
}
onBeforeUnmount(() => { disposed = true; images.value.forEach(image => releaseImage(image.localPath)) })

function removeConclusion(i) {
  conclusionItems.value.splice(i, 1)
}

async function persistDraft() {
  const updated = await updateObservation(obs.value.id, obs.value.revision, {
    ...(location.value || {}),
    location_text: locationText.value.trim() || '未填写',
    inspection_outcome: overall.value,
  })
  obs.value = updated.data.observation
  await saveConclusion(obs.value.id, obs.value.revision, {
    outcome: overall.value,
    description: overallNote.value,
    items: conclusionItems.value.map(item => ({
      image_id: item.imageId, source_detection_id: item.source_detection_id,
      final_box_json: item.final_box_json, decision: item.decision,
      final_class_code: item.final_class_code, description: item.description,
    })),
  })
  obs.value.revision++
}

function validateConclusionItems() {
  if (conclusionItems.value.some(item => !item.imageId || !images.value.some(image => image.imageId === item.imageId))) {
    uni.showToast({ title: '结论项关联的照片不存在，请重新添加', icon: 'none' })
    return false
  }
  if (conclusionItems.value.some(item => !item.final_class_code)) {
    uni.showToast({ title: '请选择每条结论的病害类别', icon: 'none' })
    return false
  }
  return true
}

async function saveDraft() {
  if (!obs.value || busy.value || disposed) return
  if (!validateConclusionItems()) return
  submitting.value = true
  try { await persistDraft(); uni.showToast({ title: '草稿已保存', icon: 'success' }) }
  catch { uni.showToast({ title: '保存失败，请检查记录状态', icon: 'none' }) }
  finally { submitting.value = false }
}

async function submit() {
  if (!obs.value || busy.value || disposed) return
  if (!validateConclusionItems()) return
  if (voluntary.value && !location.value) { uni.showToast({ title: '请先获取现场定位', icon: 'none' }); return }
  if (!locationText.value.trim()) { uni.showToast({ title: '请填写位置', icon: 'none' }); return }
  if (voluntary.value && !overallNote.value.trim()) { uni.showToast({ title: '请填写现场描述', icon: 'none' }); return }
  if (!images.value.some(image => image.role === 'OVERVIEW')) { uni.showToast({ title: '缺少概览图', icon: 'none' }); return }
  if (!overall.value) { uni.showToast({ title: '请选择总体结论', icon: 'none' }); return }
  submitting.value = true
  try {
    await persistDraft()
    await submitObservation(obs.value.id, obs.value.revision)
    uni.showToast({ title: '已提交，等待审核', icon: 'success' })
    uni.redirectTo({ url: `/pages/records/index?observationId=${obs.value.id}` })
  } catch (error) {
    uni.showToast({ title: error?.statusCode === 409 ? '记录已更新，请返回后重试' : '提交失败，请检查必填项', icon: 'none' })
  } finally { submitting.value = false }
}
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f1e8;
  padding: 24rpx 32rpx calc(320rpx + env(safe-area-inset-bottom));
}
.return-reason { padding: 24rpx; margin-bottom: 24rpx; color: #9f422d; background: #fff0e9; border-radius: 16rpx; }
.description-label { margin-top: 24rpx; font-size: 28rpx; color: #2b2b2b; }
.field {
  margin-bottom: 28rpx;
}
.field-label {
  font-size: 28rpx;
  font-weight: 600;
  color: #2b2b2b;
  margin-bottom: 12rpx;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.req {
  color: #b5543a;
  font-size: 22rpx;
  font-weight: 400;
}
.faint {
  font-size: 22rpx;
  color: #9b9b9b;
  margin-top: 8rpx;
}
.img-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16rpx;
}
.img-cell {
  position: relative;
  height: 192rpx;
  border-radius: 16rpx;
  overflow: hidden;
  background: #e9e4d8;
}
.img-cell.selected { box-shadow: inset 0 0 0 4rpx #2f6f63; }
.img-cell.selected .img { opacity: .8; }
.img-cell.add {
  display: flex;
  align-items: center;
  justify-content: center;
  color: #9b9b9b;
  font-size: 56rpx;
  border: 2rpx dashed #e6e1d5;
  background: #fbf9f3;
}
.img {
  width: 100%;
  height: 100%;
}
.del {
  position: absolute;
  top: 8rpx;
  right: 8rpx;
  width: 40rpx;
  height: 40rpx;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 24rpx;
  text-align: center;
  line-height: 40rpx;
  z-index: 2;
}
.tag-ov {
  position: absolute;
  left: 8rpx;
  bottom: 8rpx;
  font-size: 18rpx;
  background: #2f6f63;
  color: #fff;
  padding: 2rpx 10rpx;
  border-radius: 6rpx;
}
.tag-ov.detail {
  background: #6b6b6b;
}
.loc-box {
  background: #fff;
  border-radius: 16rpx;
  padding: 16rpx 20rpx;
}
.loc-line {
  display: flex;
  align-items: center;
  gap: 12rpx;
  font-size: 26rpx;
  color: #2b2b2b;
  margin-bottom: 12rpx;
}
.loc-dot {
  width: 16rpx;
  height: 16rpx;
  border-radius: 50%;
  background: #2f6f63;
}
.input {
  min-height: 88rpx;
  width: 100%;
  border: 1rpx solid #e6e1d5;
  border-radius: 16rpx;
  padding: 18rpx 20rpx;
  font-size: 28rpx;
  background: #fff;
  color: #2b2b2b;
  box-sizing: border-box;
}
textarea.input {
  min-height: 120rpx;
  margin-top: 12rpx;
}
.input-sm {
  min-height: 64rpx;
  box-sizing: border-box;
  flex: 1;
  border: 1rpx solid #e6e1d5;
  border-radius: 10rpx;
  padding: 8rpx 14rpx;
  font-size: 26rpx;
  min-width: 0;
}
.pill-row {
  display: flex;
  gap: 16rpx;
}
.pill {
  flex: 1;
  text-align: center;
  padding: 16rpx 0;
  border-radius: 40rpx;
  border: 1rpx solid #e6e1d5;
  background: #fff;
  font-size: 26rpx;
  color: #6b6b6b;
}
.pill.on {
  background: #2f6f63;
  border-color: #2f6f63;
  color: #fff;
  font-weight: 600;
}
.card {
  background: #fff;
  border-radius: 16rpx;
  padding: 8rpx 20rpx;
}
.empty-sm {
  color: #9b9b9b;
  font-size: 26rpx;
  padding: 24rpx 0;
  text-align: center;
}
.concl-item {
  display: flex;
  gap: 16rpx;
  align-items: center;
  padding: 20rpx 0;
  border-bottom: 1rpx solid #e6e1d5;
  position: relative;
}
.concl-item:last-child {
  border-bottom: none;
}
.thumb {
  width: 96rpx;
  height: 96rpx;
  border-radius: 12rpx;
  background: #d9d2c0;
  flex: none;
  overflow: hidden;
}
.row {
  display: flex;
  gap: 12rpx;
  align-items: center;
}
.grow {
  flex: 1;
}
.chip {
  font-size: 22rpx;
  padding: 4rpx 14rpx;
  border-radius: 8rpx;
  flex: none;
}
.chip.c-confirm { background: rgba(62, 155, 91, 0.12); color: #3e9b5b; }
.chip.c-modify { background: rgba(201, 138, 45, 0.12); color: #c98a2d; }
.chip.c-reject { background: rgba(138, 138, 138, 0.12); color: #8a8a8a; }
.chip.c-add { background: rgba(181, 84, 58, 0.12); color: #b5543a; }
.chip.brick { background: rgba(181, 84, 58, 0.12); color: #b5543a; }
.btn-ghost {
  background: #fff;
  color: #2f6f63;
  border: 2rpx solid #2f6f63;
  border-radius: 16rpx;
  height: 88rpx;
  line-height: 88rpx;
  font-size: 30rpx;
  font-weight: 600;
  margin-top: 16rpx;
}
.detect-note {
  font-size: 24rpx;
  color: #6b6b6b;
  background: #fff;
  border-radius: 12rpx;
  padding: 12rpx 20rpx;
  margin-bottom: 12rpx;
}
.detect-list {
  background: #fff;
  border-radius: 12rpx;
  padding: 8rpx 20rpx;
}
.detect-item {
  display: flex;
  gap: 16rpx;
  align-items: flex-start;
  flex-direction: column;
  padding: 14rpx 0;
  border-bottom: 1rpx solid #e6e1d5;
}
.detect-item:last-child {
  border-bottom: none;
}
.bottombar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  background: #fff;
  border-top: 1rpx solid #e6e1d5;
  padding: 20rpx 32rpx;
  padding-bottom: calc(20rpx + env(safe-area-inset-bottom));
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
.note {
  font-size: 22rpx;
  color: #6b6b6b;
  text-align: center;
}
</style>
