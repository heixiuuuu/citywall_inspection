<template>
  <header class="page-heading"><div class="heading-copy"><span class="page-kicker">{{ kicker }}</span><h1>{{ title }}</h1><p>{{ description }}</p></div><div v-if="$slots.default" class="page-actions"><slot /></div></header>
</template>
<script setup lang="ts">
withDefaults(defineProps<{ title: string; description: string; kicker?: string }>(), { kicker: '古城墙巡检 · 日常保护' })
</script>
<style scoped>
.page-heading { display: flex; align-items: center; justify-content: space-between; gap: var(--space-section); margin-bottom: var(--space-section); }
.page-kicker { font-size: var(--font-caption); color: var(--brand); letter-spacing: .06em; }
h1 { margin: 4px 0; color: var(--text-strong); font-size: var(--font-heading); font-weight: 600; letter-spacing: -.02em; }
p { margin: 0; font-size: var(--font-small); color: var(--text-muted); line-height: 1.7; }
.page-actions { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-tight); flex: none; }
@media (max-width: 760px) { .page-heading { align-items: flex-start; flex-direction: column; gap: var(--space-content); } }
</style>
