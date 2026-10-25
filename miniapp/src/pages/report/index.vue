<template>
  <view class="page">
    <view v-if="returnReason" class="return-reason">退回原因：{{ returnReason }}</view>
    <view class="field">
      <view class="field-label">发现位置 <text class="req">必填</text></view>
      <view class="loc-box">
        <view class="loc-line"><view class="loc-dot" />位置描述：{{ locationText || '待填写' }}</view>
        <input class="input" v-model="locationText" placeholder="填写具体位置描述" />
        <LocationCapture :value="location" @change="location = $event" />
      </view>
    </view>

    <view class="field">
      <view class="field-label">现场描述 <text class="req">必填</text></view>
      <textarea class="input" v-model="description" placeholder="描述发现的病害或异常情况" />
    </view>

    <view class="field">
      <view class="field-label">现场照片 <text class="req">概览图至少 1 张</text></view>
      <view class="img-grid">
        <view v-for="(img, i) in images" :key="img.localId" class="img-cell">
          <image :src="img.localPath" mode="aspectFill" class="img" />
          <view class="del" @tap.stop="removeImage(i)">×</view>
        </view>
        <view v-for="n in slots" :key="'add' + n" class="img-cell add" @tap="chooseImage">＋</view>
      </view>
    </view>

    <view class="field">
      <view class="field-label">现场判断</view>
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

    <view class="field">
      <view class="field-label">人工结论 <text class="req">必填</text></view>
      <view class="card">
        <view v-if="!conclusionItems.length" class="empty-sm">尚未填写结论项</view>
        <view v-for="(it, i) in conclusionItems" :key="i" class="concl-item">
          <view class="grow">
            <view class="row">
              <view class="chip c-add">补录</view>
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
    </view>

    <view class="bottombar">
      <button class="btn-ghost" :loading="submitting" @tap="saveDraft">保存草稿</button>
      <button class="btn-primary" :loading="submitting" @tap="submit">提交上报</button>
      <view class="note">主动上报将由审核员在管理端审核，可驳回</view>
    </view>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import LocationCapture from '../../shared/ui/LocationCapture.vue'
import {
  createObservation,
  addImage,
  updateObservation,
  saveConclusion,
  submitObservation,
  removeImage as deleteImage,
  loadObservation,
} from '../../shared/api/observation'

const outcomes = [
  { value: 'NORMAL', label: '正常' },
  { value: 'SUSPECTED', label: '疑似病害' },
  { value: 'UNDETERMINED', label: '不确定' },
  { value: 'UNREACHABLE', label: '不可达' },
]

const obs = ref(null)
const images = ref([])
const locationText = ref('')
const location = ref(null)
const description = ref('')
const outcome = ref(null)
const overall = ref('SUSPECTED')
const conclusionItems = ref([])
const submitting = ref(false)
const returnReason = ref('')

const slots = computed(() => Math.max(0, 6 - images.value.length))

async function boot() {
  try {
    const res = await createObservation('VOLUNTARY', null, locationText.value || '未填写')
    obs.value = res.data.observation
  } catch (e) {
    uni.showToast({ title: '创建记录失败', icon: 'none' })
  }
}
onLoad(async query => {
  if (!query.observationId) { await boot(); return }
  try {
    const { evidence, images: savedImages } = await loadObservation(Number(query.observationId))
    const record = evidence.observation
    if (record.source !== 'VOLUNTARY' || !['DRAFT', 'RETURNED'].includes(record.status)) {
      uni.redirectTo({ url: `/pages/records/index?observationId=${record.id}` })
      return
    }
    obs.value = record
    images.value = savedImages
    locationText.value = record.location_text
    outcome.value = record.inspection_outcome
    if (record.latitude != null && record.longitude != null) {
      location.value = { latitude: record.latitude, longitude: record.longitude, accuracy_m: record.accuracy_m, captured_at: record.captured_at }
    }
    const latest = evidence.conclusions[evidence.conclusions.length - 1]
    if (latest) {
      overall.value = latest.outcome
      description.value = latest.description
      conclusionItems.value = evidence.items
        .filter(item => item.conclusion_revision_id === latest.id && images.value.some(image => image.imageId === item.image_id))
        .map(item => ({ ...item, imageId: item.image_id }))
    }
    if (record.status === 'RETURNED') returnReason.value = evidence.reviews[evidence.reviews.length - 1]?.reason || '请补充现场信息'
  } catch { uni.showToast({ title: '加载记录失败', icon: 'none' }) }
})

function chooseImage() {
  uni.chooseImage({
    count: 1,
    sizeType: ['compressed'],
    sourceType: ['camera', 'album'],
    success: (res) => {
      const localPath = res.tempFilePaths[0]
      if (!obs.value) return
      const role = images.value.some(image => image.role === 'OVERVIEW') ? 'DETAIL' : 'OVERVIEW'
      addImage(obs.value.id, localPath, role)
        .then((r) => {
          images.value.push({ localId: `img_${Date.now()}`, localPath, imageId: r.data.image.id, role })
        })
        .catch(() => {
          uni.showToast({ title: '上传失败', icon: 'none' })
        })
    },
  })
}

async function removeImage(i) {
  const image = images.value[i]
  try {
    await deleteImage(obs.value.id, image.imageId)
    images.value.splice(i, 1)
    conclusionItems.value = conclusionItems.value.filter(item => item.imageId !== image.imageId)
  } catch { uni.showToast({ title: '照片移除失败', icon: 'none' }) }
}

function addConclusion() {
  const firstImg = images.value[0]
  conclusionItems.value.push({
    imageId: firstImg ? firstImg.imageId : null,
    decision: 'ADD',
    final_class_code: '',
    description: '',
  })
}

function removeConclusion(i) {
  conclusionItems.value.splice(i, 1)
}

async function persistDraft() {
  const updated = await updateObservation(obs.value.id, obs.value.revision, {
    ...(location.value || {}), location_text: locationText.value.trim() || '未填写',
    inspection_outcome: outcome.value || overall.value,
  })
  obs.value = updated.data.observation
  await saveConclusion(obs.value.id, obs.value.revision, {
    outcome: overall.value, description: description.value.trim(),
    items: conclusionItems.value.filter(item => item.imageId && item.final_class_code).map(item => ({
      image_id: item.imageId, decision: item.decision, final_class_code: item.final_class_code,
      source_detection_id: item.source_detection_id, final_box_json: item.final_box_json, description: item.description,
    })),
  })
  obs.value.revision++
}
async function saveDraft() {
  if (!obs.value || submitting.value) return
  submitting.value = true
  try { await persistDraft(); uni.showToast({ title: '草稿已保存', icon: 'success' }) }
  catch { uni.showToast({ title: '保存失败，请检查记录状态', icon: 'none' }) }
  finally { submitting.value = false }
}
async function submit() {
  if (!obs.value || submitting.value) return
  if (!location.value) { uni.showToast({ title: '请先获取现场定位', icon: 'none' }); return }
  if (!locationText.value.trim()) { uni.showToast({ title: '请填写位置', icon: 'none' }); return }
  if (!description.value.trim()) { uni.showToast({ title: '请填写现场描述', icon: 'none' }); return }
  if (!images.value.some(image => image.role === 'OVERVIEW')) { uni.showToast({ title: '缺少概览图', icon: 'none' }); return }
  submitting.value = true
  try {
    await persistDraft()
    await submitObservation(obs.value.id, obs.value.revision)
    uni.showToast({ title: '已提交，等待审核', icon: 'success' })
    setTimeout(() => uni.navigateBack(), 800)
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
}
.req {
  color: #b5543a;
  font-size: 22rpx;
  font-weight: 400;
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
  min-height: 88rpx;
  border: 1rpx solid #e6e1d5;
  border-radius: 16rpx;
  padding: 18rpx 20rpx;
  font-size: 28rpx;
  background: #fff;
  color: #2b2b2b;
  box-sizing: border-box;
}
textarea.input {
  min-height: 140rpx;
}
.input-sm {
  flex: 1;
  min-height: 64rpx;
  box-sizing: border-box;
  border: 1rpx solid #e6e1d5;
  border-radius: 10rpx;
  padding: 8rpx 14rpx;
  font-size: 26rpx;
  min-width: 0;
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
.chip.c-add { background: rgba(181, 84, 58, 0.12); color: #b5543a; }
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
