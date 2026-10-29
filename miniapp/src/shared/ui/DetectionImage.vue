<template>
  <view class="evidence-photo" @tap="$emit('preview', src)">
    <image :src="src" mode="widthFix" class="photo" @load="loaded" @error="size = null" />
    <view v-if="size" class="boxes">
      <view v-for="(box, index) in boxes" :key="box.id" class="box" :style="boxStyle(box)">
        <text class="number">{{ index + 1 }}</text>
      </view>
      <view v-for="(box, index) in manualBoxes" :key="'manual-' + index" class="box manual" :style="boxStyle(box)"><text class="number">人工</text></view>
    </view>
    <view v-if="drawing && size" class="draw-surface" @tap.stop @touchstart.stop.prevent="startDraw" @touchmove.stop.prevent="moveDraw" @touchend.stop.prevent="endDraw">
      <view v-if="selection" class="box manual" :style="boxStyle(selection)" />
    </view>
  </view>
</template>
<script setup>
import { getCurrentInstance, ref, watch } from 'vue'
const props = defineProps({ src: { type: String, required: true }, boxes: { type: Array, default: () => [] }, manualBoxes: { type: Array, default: () => [] }, drawing: Boolean })
const emit = defineEmits(['preview', 'draw'])
const size = ref(null)
const selection = ref(null)
const instance = getCurrentInstance()
let start = null
let bounds = null
watch(() => props.src, () => { size.value = null; selection.value = null; start = null })
watch(() => props.drawing, value => { if (!value) { selection.value = null; start = null } })
function loaded(event) {
  const { width, height } = event.detail
  size.value = width > 0 && height > 0 ? { width, height } : null
}
function boxStyle(box) {
  const { width, height } = size.value
  const clamp = (value, max) => Math.max(0, Math.min(max, Number(value) || 0))
  const left = clamp(box.x1, width), top = clamp(box.y1, height)
  const right = clamp(box.x2, width), bottom = clamp(box.y2, height)
  return { left: `${left / width * 100}%`, top: `${top / height * 100}%`, width: `${Math.max(0, right - left) / width * 100}%`, height: `${Math.max(0, bottom - top) / height * 100}%` }
}
function point(touch) {
  if (!touch || !bounds || !size.value) return null
  const x = Math.max(0, Math.min(1, (touch.clientX - bounds.left) / bounds.width))
  const y = Math.max(0, Math.min(1, (touch.clientY - bounds.top) / bounds.height))
  return { x: Math.round(x * size.value.width), y: Math.round(y * size.value.height) }
}
function startDraw(event) {
  const current = event.touches?.[0]
  if (!current) return
  const touch = { clientX: current.clientX, clientY: current.clientY }
  uni.createSelectorQuery().in(instance.proxy).select('.draw-surface').boundingClientRect(rect => {
    if (!rect?.width || !rect?.height || !props.drawing) return
    bounds = rect
    start = point(touch)
    selection.value = start ? { x1: start.x, y1: start.y, x2: start.x, y2: start.y } : null
  }).exec()
}
function moveDraw(event) {
  const end = point(event.touches?.[0])
  if (!start || !end) return
  selection.value = { x1: Math.min(start.x, end.x), y1: Math.min(start.y, end.y), x2: Math.max(start.x, end.x), y2: Math.max(start.y, end.y) }
}
function endDraw(event) {
  if (event.changedTouches?.[0]) moveDraw({ touches: event.changedTouches })
  const box = selection.value
  start = null
  if (box && box.x2 > box.x1 && box.y2 > box.y1) emit('draw', box)
  else { selection.value = null; uni.showToast({ title: '请拖动框选病害位置', icon: 'none' }) }
}
</script>
<style scoped>
.evidence-photo { --box-color: #f4c95d; --box-label: #302b20; --photo-radius: 12rpx; position: relative; overflow: hidden; border-radius: var(--photo-radius); }
.photo { display: block; width: 100%; }
.boxes { position: absolute; inset: 0; pointer-events: none; }
.draw-surface { position: absolute; inset: 0; touch-action: none; }
.manual { border-color: #e15d48; background: rgba(225, 93, 72, .12); pointer-events: none; }
.box { position: absolute; box-sizing: border-box; border: 2px solid var(--box-color); }
.number { position: absolute; top: 0; left: 0; padding: 0 6rpx; color: var(--box-label); background: var(--box-color); font-size: 22rpx; line-height: 1.4; font-variant-numeric: tabular-nums; }
</style>
