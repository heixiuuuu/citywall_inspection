<template>
  <el-select :model-value="modelValue" filterable remote clearable :remote-method="search" :loading="loading" placeholder="输入任务名称搜索" style="width: 100%" @visible-change="onVisible" @update:model-value="updateValue">
    <el-option v-if="selected && !options.some(task => task.id === selected?.id)" :value="selected.id" :label="`${selected.title}（#${selected.id}）`" />
    <el-option v-for="task in options" :key="task.id" :value="task.id" :label="`${task.title}（#${task.id}）`" />
    <template #empty><div class="empty">{{ failed ? '任务加载失败' : '暂无匹配任务' }}<el-button v-if="failed" text type="primary" @click="search(keyword)">重新加载</el-button></div></template>
    <template v-if="options.length < total" #footer><el-button text type="primary" :loading="loading" @click="loadMore">加载更多任务</el-button></template>
  </el-select>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { fetchTask, fetchTasks, type Task } from '@/shared/api/task'

const props = defineProps<{ modelValue?: number }>()
const emit = defineEmits<{ 'update:modelValue': [value: number | undefined] }>()
const options = ref<Task[]>([])
const selected = ref<Task | null>(null)
const keyword = ref('')
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const failed = ref(false)
let generation = 0
function onVisible(visible: boolean) { if (visible && !options.value.length) void search('') }
function updateValue(value: unknown) { emit('update:modelValue', typeof value === 'number' ? value : undefined) }
watch(() => props.modelValue, async id => {
  if (!id) { selected.value = null; return }
  const option = options.value.find(task => task.id === id)
  if (option) { selected.value = option; return }
  try { const result = await fetchTask(id); if (props.modelValue === id) selected.value = result.task }
  catch { if (props.modelValue === id) selected.value = null }
}, { immediate: true })

async function search(value: string) {
  keyword.value = value.trim()
  page.value = 1
  await load(false)
}
async function loadMore() {
  if (loading.value) return
  page.value++
  await load(true)
}
async function load(append: boolean) {
  const current = ++generation
  loading.value = true
  failed.value = false
  try {
    const result = await fetchTasks({ keyword: keyword.value, page: page.value, page_size: 20 })
    if (current !== generation) return
    options.value = append ? [...options.value, ...result.tasks ?? []] : result.tasks ?? []
    total.value = result.total
  } catch {
    if (current !== generation) return
    failed.value = true
    if (append) page.value--
    else options.value = []
  } finally { if (current === generation) loading.value = false }
}
</script>

<style scoped>
.empty { padding: 12px var(--space-content); text-align: center; color: var(--text-muted); }
</style>
