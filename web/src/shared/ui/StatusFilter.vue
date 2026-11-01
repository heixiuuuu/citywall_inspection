<template>
  <div class="status-filter" role="group" :aria-label="label"><button v-for="item in choices" :key="item.value" type="button" :aria-pressed="modelValue === item.value" :class="{ selected: modelValue === item.value }" @click="$emit('update:modelValue', modelValue === item.value ? '' : item.value)"><span class="selection-mark" aria-hidden="true">{{ modelValue === item.value ? '✓' : '' }}</span>{{ item.label }}</button></div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
const props = withDefaults(defineProps<{ modelValue: string; options: readonly { value: string; label: string }[]; label?: string }>(), { label: '状态筛选' })
defineEmits<{ 'update:modelValue': [value: string] }>()
const choices = computed(() => [{ value: '', label: '全部' }, ...props.options])
</script>
<style scoped>
.status-filter { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-tight); margin-bottom: var(--space-content); }
button { display: inline-flex; align-items: center; gap: 4px; border: none; border-radius: var(--radius-control); padding: 7px 12px; background: var(--surface-subtle); color: var(--text-muted); font: inherit; font-size: var(--font-small); cursor: pointer; transition: background-color var(--motion-fast), color var(--motion-fast); }
button:hover { background: var(--surface-hover); color: var(--brand); }
button.selected { background: var(--brand); color: var(--surface); }
.selection-mark { width: 12px; font-size: 11px; }
</style>
