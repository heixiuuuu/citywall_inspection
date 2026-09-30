<template>
  <view class="page">
    <view class="field">
      <view class="field-label">
        现场照片 <text class="req">概览图至少 1 张</text>
      </view>
      <view class="img-grid">
        <view
          v-for="(img, i) in images"
          :key="img.localId"
          class="img-cell"
        >
          <image :src="img.localPath" mode="aspectFill" class="img" />
          <view class="del" @tap.stop="removeImage(i)">×</view>
          <view class="tag-ov" :class="{ detail: img.role === 'DETAIL' }">
            {{ img.role === 'OVERVIEW' ? '概览' : '细节' }}
          </view>
        </view>
        <view v-for="n in slots" :key="'add' + n" class="img-cell add" @tap="chooseImage(n)">＋</view>
      </view>
      <view class="faint">拍照或从相册选择 · 首张标记为概览图，可切换角色</view>
    </view>

    <view class="field">
      <view class="field-label">位置 <text class="req">必填</text></view>
      <view class="loc-box">
        <view class="loc-line"><view class="loc-dot" />已定位：{{ locationText || '待填写' }}</view>
        <input class="input" v-model="locationText" placeholder="填写具体位置描述" />
      </view>
    </view>

    <view class="field">
      <view class="field-label">现场判断 <text class="req">必选</text></view>
      <view class="pill-row">
        <view
          v-for="o in outcomes"
          :key="o.value"
          class="pill"
          :class="{ on: outcome === o.value }"
          @tap="outcome = o.value"
        >
          {{ o.label }}
        </view>
      </view>
    </view>

    <view class="field" v-if="detections.length">
      <view class="field-label">
        视觉识别 <text class="faint">候选仅作参考，不替代人工结论</text>
      </view>
      <view class="detect-note">
        检测到 {{ detections.length }} 个候选 · 模型 {{ modelVersion || 'yolo_baseline_001' }}
      </view>
      <view class="detect-list">
        <view v-for="d in detections" :key="d.id" class="detect-item">
          <view class="chip brick">{{ d.class_code }} {{ d.confidence.toFixed(2) }}</view>
          <text class="faint">框 ({{ d.x1 }},{{ d.y1 }})-({{ d.x2 }},{{ d.y2 }})</text>
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
              <input class="input-sm" v-model="it.final_class_code" placeholder="病害类别" />
            </view>
            <input class="input-sm" v-model="it.description" placeholder="补充描述（可选）" />
          </view>
          <view class="del" @tap="removeConclusion(i)">×</view>
        </view>
        <button class="btn-ghost" @tap="addConclusion">＋ 添加结论项</button>
      </view>
    </view>

    <view class="field">
      <view class="field-label">总体结论</view>
      <view class="pill-row">
        <view
          v-for="o in outcomes"
          :key="'o' + o.value"
          class="pill"
          :class="{ on: overall === o.value }"
          @tap="overall = o.value"
        >
          {{ o.label }}
        </view>
      </view>
      <textarea class="input" v-model="overallNote" placeholder="补充说明（可选）" />
    </view>

    <view class="field">
      <button class="btn-ghost" :loading="recognizing" @tap="runRecognition">触发视觉识别</button>
    </view>

    <view class="bottombar">
      <button class="btn-primary" :loading="submitting" @tap="submit">提交记录</button>
      <view class="note">保存草稿后可继续补充 · 提交后进入待审核</view>
    </view>
  </view>
</template>

<script setup>
import { computed, onLoad, ref } from 'vue'
import {
  createObservation,
  addImage,
  updateObservation,
  recognize,
  saveConclusion,
  submitObservation,
} from '../../shared/api/observation'

const outcomes = [
  { value: 'NORMAL', label: '正常' },
  { value: 'SUSPECTED', label: '疑似病害' },
  { value: 'UNDETERMINED', label: '不确定' },
  { value: 'UNREACHABLE', label: '不可达' },
]

const taskId = ref(null)
const taskPointId = ref(null)
const pointName = ref('')
const obs = ref(null)
const images = ref([])
const locationText = ref('')
const outcome = ref(null)
const overall = ref('SUSPECTED')
const overallNote = ref('')
const conclusionItems = ref([])
const detections = ref([])
const modelVersion = ref('')
const recognizing = ref(false)
const submitting = ref(false)
const lastDetectionImageId = ref(null)

onLoad((query) => {
  taskId.value = Number(query.taskId)
  taskPointId.value = Number(query.taskPointId)
  pointName.value = decodeURIComponent(query.name || '点位')
  locationText.value = pointName.value
  boot()
})

const slots = computed(() => Math.max(0, 6 - images.value.length))

async function boot() {
  try {
    const res = await createObservation('TASK', taskPointId.value, locationText.value)
    obs.value = res.data.observation
  } catch (e) {
    if (e && e.statusCode === 409) {
      uni.showToast({ title: '该点位已有记录', icon: 'none' })
    } else {
      uni.showToast({ title: '创建记录失败', icon: 'none' })
    }
  }
}

function chooseImage(slotIndex) {
  uni.chooseImage({
    count: 1,
    sizeType: ['compressed'],
    sourceType: ['camera', 'album'],
    success: (res) => {
      const localPath = res.tempFilePaths[0]
      if (!obs.value) {
        uni.showToast({ title: '记录尚未创建', icon: 'none' })
        return
      }
      // 首张为概览图；slotIndex >= 1 用户可指定（MVP：新图默认为细节图，首张概览）。
      const role = images.value.length === 0 ? 'OVERVIEW' : 'DETAIL'
      addImage(obs.value.id, localPath, role)
        .then((r) => {
          const image = r.data.image
          images.value.push({ localId: `img_${Date.now()}`, localPath, imageId: image.id, role })
          if (role === 'OVERVIEW') {
            // 概览图就绪后自动做一次识别作为候选参考。
            runRecognitionOn(image.id)
          }
          uni.showToast({ title: '上传成功', icon: 'success' })
        })
        .catch(() => {
          uni.showToast({ title: '上传失败', icon: 'none' })
        })
    },
  })
}

function removeImage(i) {
  images.value.splice(i, 1)
}

async function runRecognition() {
  if (!lastDetectionImageId.value) {
    uni.showToast({ title: '请先上传概览图', icon: 'none' })
    return
  }
  runRecognitionOn(lastDetectionImageId.value)
}

async function runRecognitionOn(imageId) {
  recognizing.value = true
  try {
    const res = await recognize(imageId)
    const attempt = res.data.attempt
    const dets = res.data.detections || []
    lastDetectionImageId.value = imageId
    detections.value = dets
    modelVersion.value = attempt.model_version
    if (attempt.status === 'FAILED') {
      uni.showToast({ title: `识别失败：${attempt.error_code || '未知错误'}`, icon: 'none' })
    } else if (!dets.length) {
      uni.showToast({ title: '未检出候选', icon: 'none' })
    }
  } catch (e) {
    uni.showToast({ title: '识别服务不可用', icon: 'none' })
  } finally {
    recognizing.value = false
  }
}

function decisionText(d) {
  return { CONFIRM: '确认', MODIFY: '修改', REJECT: '删除', ADD: '补录' }[d] || d
}

function addConclusion() {
  const firstImg = images.value.find((i) => i.role === 'OVERVIEW') || images.value[0]
  conclusionItems.value.push({
    imageId: firstImg ? firstImg.imageId : null,
    thumb: firstImg ? firstImg.localPath : '',
    decision: 'ADD',
    final_class_code: '',
    description: '',
  })
}

function removeConclusion(i) {
  conclusionItems.value.splice(i, 1)
}

async function submit() {
  if (!obs.value) return
  if (!locationText.value.trim()) {
    uni.showToast({ title: '请填写位置', icon: 'none' })
    return
  }
  if (!images.value.some((i) => i.role === 'OVERVIEW')) {
    uni.showToast({ title: '缺少概览图', icon: 'none' })
    return
  }
  if (!overall.value) {
    uni.showToast({ title: '请选择总体结论', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    // 1) 更新位置与现场判断。
    let revision = obs.value.revision
    await updateObservation(obs.value.id, revision, {
      location_text: locationText.value.trim(),
      inspection_outcome: outcome.value || overall.value,
    })
    revision++

    // 2) 保存人工结论（整体替换）。
    await saveConclusion(obs.value.id, revision, {
      outcome: overall.value,
      description: overallNote.value,
      items: conclusionItems.value
        .filter((it) => it.imageId && it.final_class_code)
        .map((it) => ({
          image_id: it.imageId,
          decision: it.decision,
          final_class_code: it.final_class_code,
          description: it.description,
        })),
    })
    revision++

    // 3) 提交记录。
    await submitObservation(obs.value.id, revision)
    uni.showToast({ title: '已提交，等待审核', icon: 'success' })
    setTimeout(() => uni.navigateBack(), 800)
  } catch (e) {
    if (e && e.statusCode === 409) {
      uni.showToast({ title: '数据已更新，请重试', icon: 'none' })
    } else {
      uni.showToast({ title: '提交失败，请检查必填项', icon: 'none' })
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f1e8;
  padding: 24rpx 32rpx 200rpx;
}
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
  align-items: center;
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
