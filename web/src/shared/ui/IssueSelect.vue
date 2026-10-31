<template>
  <el-select :model-value="modelValue" multiple filterable remote clearable :remote-method="search" :loading="loading" placeholder="搜索未关闭的问题名称或位置" style="width: 100%" @visible-change="onVisible" @update:model-value="updateValue">
    <el-option v-for="item in selectedOutsideResults" :key="item.id" :value="item.id" :label="`${item.title}（#${item.id}）`" />
    <el-option v-for="item in options" :key="item.id" :value="item.id" :label="`${item.title}（#${item.id}）`"><span>{{ item.title }}</span><small class="option-status">{{ ISSUE_STATUS[item.status]?.label }}</small></el-option>
    <template #empty><div class="empty">{{ failed ? '问题加载失败' : '没有匹配的未关闭问题' }}<el-button v-if="failed" text type="primary" @click="search(keyword)">重新加载</el-button></div></template>
    <template v-if="options.length < total" #footer><el-button text type="primary" :loading="loading" @click="loadMore">加载更多问题</el-button></template>
  </el-select>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { fetchIssues, ISSUE_STATUS, type Issue } from '@/shared/api/issue'

const props = defineProps<{ modelValue: number[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: number[]] }>()
const options = ref<Issue[]>([])
const cache = ref(new Map<number, Issue>())
const keyword = ref('')
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const failed = ref(false)
let generation = 0
const selectedOutsideResults = computed(() => [...cache.value.values()].filter(item => props.modelValue.includes(item.id) && !options.value.some(option => option.id === item.id)))
function onVisible(visible: boolean) { if (visible && !options.value.length) void search('') }
function updateValue(value: unknown) { emit('update:modelValue', Array.isArray(value) ? value.filter((id): id is number => typeof id === 'number') : []) }
async function search(value: string) { keyword.value = value.trim(); page.value = 1; await load(false) }
async function loadMore() { if (loading.value) return; page.value++; await load(true) }
async function load(append: boolean) {
  const current = ++generation
  loading.value = true
  failed.value = false
  try {
    const result = await fetchIssues({ keyword: keyword.value, active_only: true, page: page.value, page_size: 20 })
    if (current !== generation) return
    options.value = append ? [...options.value, ...result.items] : result.items
    cache.value = new Map([...cache.value, ...result.items.map(item => [item.id, item] as const)])
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
.option-status { float: right; margin-left: var(--space-content); color: var(--text-muted); }
</style>
