<template>
  <view class="evidence-photo" @tap="$emit('preview', src)">
    <image :src="src" mode="widthFix" class="photo" @load="loaded" @error="size = null" />
    <view v-if="size" class="boxes">
      <view v-for="(box, index) in boxes" :key="box.id" class="box" :style="boxStyle(box)">
        <text class="number">{{ index + 1 }}</text>
      </view>
    </view>
  </view>
</template>
<script setup>
import { ref, watch } from 'vue'
const props = defineProps({ src: { type: String, required: true }, boxes: { type: Array, default: () => [] } })
defineEmits(['preview'])
const size = ref(null)
watch(() => props.src, () => { size.value = null })
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
</script>
<style scoped>
.evidence-photo { --box-color: #f4c95d; --box-label: #302b20; --photo-radius: 12rpx; position: relative; overflow: hidden; border-radius: var(--photo-radius); }
.photo { display: block; width: 100%; }
.boxes { position: absolute; inset: 0; pointer-events: none; }
.box { position: absolute; box-sizing: border-box; border: 2px solid var(--box-color); }
.number { position: absolute; top: 0; left: 0; padding: 0 6rpx; color: var(--box-label); background: var(--box-color); font-size: 22rpx; line-height: 1.4; font-variant-numeric: tabular-nums; }
</style>
