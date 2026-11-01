<template>
  <div class="list-feedback" :role="error ? 'alert' : 'status'"><div class="feedback-symbol" :class="{ failed: error }"><AppIcon :name="error ? 'retry' : icon" /></div><strong>{{ error ? '暂时无法加载' : title }}</strong><p>{{ error || description }}</p><el-button v-if="error" @click="$emit('retry')">重新加载</el-button><slot v-else /></div>
</template>
<script setup lang="ts">
import AppIcon from './AppIcon.vue'
withDefaults(defineProps<{ error?: string; title: string; description: string; icon?: string }>(), { error: '', icon: 'search' })
defineEmits<{ retry: [] }>()
</script>
<style scoped>
.list-feedback { padding: var(--space-empty) var(--space-content); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-tight); white-space: normal; line-height: 1.7; }
.feedback-symbol { --icon-size: 26px; width: var(--empty-symbol-size); height: var(--empty-symbol-size); display: grid; place-items: center; border-radius: var(--radius-panel); background: var(--surface-sunken); color: var(--brand); margin-bottom: var(--space-tight); }
.feedback-symbol.failed { color: var(--feedback-warning); background: var(--feedback-warning-bg); }
strong { color: var(--text-strong); font-weight: 500; font-size: var(--font-body); }
p { margin: 0 0 var(--space-tight); color: var(--text-muted); font-size: var(--font-small); max-width: 420px; text-align: center; }
</style>
