<template>
  <picker :range="options" range-key="label" :value="index" :disabled="disabled" @change="change">
    <view class="category">{{ categoryLabel(modelValue) }}<text v-if="!disabled"> ▾</text></view>
  </picker>
</template>
<script setup>
import { computed } from 'vue'
import { categories, categoryLabel } from './categories'
const props = defineProps({ modelValue: { type: String, default: '' }, disabled: Boolean })
const emit = defineEmits(['update:modelValue'])
const options = [{ code: '', label: '选择病害类别' }, ...categories]
const index = computed(() => Math.max(0, options.findIndex(item => item.code === props.modelValue)))
function change(event) { emit('update:modelValue', options[Number(event.detail.value)].code) }
</script>
<style scoped>
.category { min-height: 64rpx; box-sizing: border-box; padding: 12rpx 14rpx; border: 1rpx solid #e6e1d5; border-radius: 10rpx; font-size: 26rpx; background: white; }
</style>
