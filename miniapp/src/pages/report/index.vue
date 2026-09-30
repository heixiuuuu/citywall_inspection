<template>
  <view class="page">
    <view class="field">
      <view class="field-label">发现位置 <text class="req">必填</text></view>
      <view class="loc-box">
        <view class="loc-line"><view class="loc-dot" />已定位：{{ locationText || '待填写' }}</view>
        <input class="input" v-model="locationText" placeholder="填写具体位置描述" />
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
      <button class="btn-primary" :loading="submitting" @tap="submit">提交上报</button>
      <view class="note">主动上报将由审核员在管理端审核，可驳回</view>
    </view>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import {
  createObservation,
  addImage,
  updateObservation,
  saveConclusion,
  submitObservation,
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
const description = ref('')
const outcome = ref(null)
const overall = ref('SUSPECTED')
const conclusionItems = ref([])
const submitting = ref(false)

const slots = computed(() => Math.max(0, 6 - images.value.length))

async function boot() {
  try {
    const res = await createObservation('VOLUNTARY', null, locationText.value || '未填写')
    obs.value = res.data.observation
  } catch (e) {
    uni.showToast({ title: '创建记录失败', icon: 'none' })
  }
}
boot()

function chooseImage() {
  uni.chooseImage({
    count: 1,
    sizeType: ['compressed'],
    sourceType: ['camera', 'album'],
    success: (res) => {
      const localPath = res.tempFilePaths[0]
      if (!obs.value) return
      const role = images.value.length === 0 ? 'OVERVIEW' : 'DETAIL'
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

function removeImage(i) {
  images.value.splice(i, 1)
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

async function submit() {
  if (!obs.value) return
  if (!locationText.value.trim()) {
    uni.showToast({ title: '请填写位置', icon: 'none' })
    return
  }
  if (!description.value.trim()) {
    uni.showToast({ title: '请填写现场描述', icon: 'none' })
    return
  }
  if (!images.value.some((i) => i.role === 'OVERVIEW')) {
    uni.showToast({ title: '缺少概览图', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    let revision = obs.value.revision
    await updateObservation(obs.value.id, revision, {
      location_text: locationText.value.trim(),
      inspection_outcome: outcome.value || overall.value,
    })
    revision++

    await saveConclusion(obs.value.id, revision, {
      outcome: overall.value,
      description: description.value.trim(),
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

    await submitObservation(obs.value.id, revision)
    uni.showToast({ title: '已提交，等待审核', icon: 'success' })
    setTimeout(() => uni.navigateBack(), 800)
  } catch (e) {
    uni.showToast({ title: e && e.statusCode === 409 ? '数据已更新，请重试' : '提交失败', icon: 'none' })
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
