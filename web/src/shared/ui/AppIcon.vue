<template>
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" class="app-icon"><path v-for="(path, i) in paths" :key="i" :d="path" /></svg>
</template>
<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{ name: string }>()
const icons: Record<string, string[]> = {
  wall: ['M3 21V6h4v4h3V6h4v4h3V6h4v15H3Z', 'M9 21v-5a3 3 0 0 1 6 0v5'],
  overview: ['M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z'],
  tasks: ['M9 4H5v17h14V4h-4', 'M9 3h6v4H9zM8 12h8M8 16h5'],
  reviews: ['M9 3H5v18h14V3h-4', 'M9 2h6v5H9zM8 13l3 3 5-6'],
  issues: ['M5 21V3', 'M5 4c5-4 9 4 15 0v10c-6 4-10-4-15 0'],
  records: ['M3 7h18v4H3zM5 11v10h14V11M9 15h6M7 3h10'],
  reports: ['M5 3h9l5 5v13H5zM14 3v6h5M8 13h8M8 17h6'],
  points: ['M3 6l6-3 6 3 6-3v15l-6 3-6-3-6 3V6ZM9 3v15M15 6v15'],
  audit: ['M12 8v5l3 2', 'M4 5v5h5', 'M4 10a8 8 0 1 1 0 6'],
  search: ['M21 21l-5-5', 'M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0'],
  retry: ['M20 4v6h-6M4 20v-6h6', 'M20 10a8 8 0 0 0-14-5M4 14a8 8 0 0 0 14 5'],
  check: ['M5 12l4 4L19 6'],
}
const paths = computed(() => icons[props.name] ?? icons.records)
</script>
<style scoped>
.app-icon { width: var(--icon-size); height: var(--icon-size); flex: none; vertical-align: -.125em; }
</style>
