<template>
  <view>
    <picker :range="options" range-key="label" :value="index" :disabled="disabled" @change="change">
      <view class="category">{{ categoryLabel(modelValue) }}<text v-if="!disabled"> ▾</text></view>
    </picker>
    <view v-if="modelDependent" class="hint">剥落识别可能误检或漏检，请人工确认，也可手动补录。</view>
  </view>
</template>
<script setup>
import { computed } from 'vue'
import { categories, categoryLabel } from './categories'
const props = defineProps({ modelValue: { type: String, default: '' }, disabled: Boolean })
const emit = defineEmits(['update:modelValue'])
const options = [{ code: '', label: '选择病害类别' }, ...categories.map(item => ({ ...item, label: item.modelDependent ? `${item.label}（可人工补录）` : item.label }))]
const index = computed(() => Math.max(0, options.findIndex(item => item.code === props.modelValue)))
const modelDependent = computed(() => categories.find(item => item.code === props.modelValue)?.modelDependent)
function change(event) { emit('update:modelValue', options[Number(event.detail.value)].code) }
</script>
<style scoped>
.category { min-height: 64rpx; box-sizing: border-box; padding: 12rpx 14rpx; border: 1rpx solid #e6e1d5; border-radius: 10rpx; font-size: 26rpx; background: white; }
.hint { margin-top: 8rpx; color: #6b6b6b; font-size: 22rpx; line-height: 1.5; }
</style>
