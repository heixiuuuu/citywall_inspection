<template>
  <section ref="container" v-loading="loading || rendering" class="pdf-reader" aria-label="报告阅读器">
    <div v-if="totalPages" class="reader-controls">
      <el-button size="small" :disabled="pageNumber <= 1 || rendering" @click="pageNumber--">上一页</el-button><span aria-live="polite">第 {{ pageNumber }} / {{ totalPages }} 页</span><el-button size="small" :disabled="pageNumber >= totalPages || rendering" @click="pageNumber++">下一页</el-button>
      <div class="zoom-controls"><el-button size="small" :disabled="zoom <= .75 || rendering" aria-label="缩小报告" @click="zoom = Math.max(.75, zoom - .25)">−</el-button><span>{{ Math.round(zoom * 100) }}%</span><el-button size="small" :disabled="zoom >= 2 || rendering" aria-label="放大报告" @click="zoom = Math.min(2, zoom + .25)">＋</el-button><el-button size="small" :disabled="rendering" @click="zoom = 1">适合宽度</el-button></div>
    </div>
    <ListFeedback v-if="error" :error="error" title="报告预览" description="" icon="reports" @retry="load" />
    <div v-show="!error" class="page-viewport"><p v-if="!totalPages" class="reader-note">正在读取报告…</p><canvas ref="canvas" v-show="totalPages" role="img" :aria-label="`报告第 ${pageNumber} 页，共 ${totalPages} 页`" /></div>
  </section>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { getDocument, GlobalWorkerOptions, type PDFDocumentLoadingTask, type PDFDocumentProxy, type RenderTask } from 'pdfjs-dist'
import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'
import ListFeedback from './ListFeedback.vue'

GlobalWorkerOptions.workerSrc = workerUrl
const props = defineProps<{ url: string }>()
const container = ref<HTMLElement>()
const canvas = ref<HTMLCanvasElement>()
const pageNumber = ref(1)
const totalPages = ref(0)
const zoom = ref(1)
const loading = ref(false)
const rendering = ref(false)
const error = ref('')
let document: PDFDocumentProxy | undefined
let loadingTask: PDFDocumentLoadingTask | undefined
let renderTask: RenderTask | undefined
let loadGeneration = 0
let renderGeneration = 0
let resizeObserver: ResizeObserver | undefined

async function load() {
  const current = ++loadGeneration
  renderGeneration++
  renderTask?.cancel()
  void loadingTask?.destroy().catch(() => {})
  document = undefined
  totalPages.value = 0
  pageNumber.value = 1
  error.value = ''
  loading.value = true
  try {
    const task = getDocument({ url: props.url, useSystemFonts: true })
    loadingTask = task
    const loaded = await task.promise
    if (current !== loadGeneration) { void task.destroy().catch(() => {}); return }
    document = loaded
    totalPages.value = loaded.numPages
    await nextTick()
    await render()
  } catch { if (current === loadGeneration) error.value = '报告内容暂时无法显示，请重新加载或下载 PDF 后查看。' }
  finally { if (current === loadGeneration) loading.value = false }
}

async function render() {
  if (!document || !canvas.value || !container.value) return
  const current = ++renderGeneration
  const activeDocument = document
  const previous = renderTask
  previous?.cancel()
  rendering.value = true
  try {
    await previous?.promise.catch(() => {})
    const page = await activeDocument.getPage(pageNumber.value)
    if (current !== renderGeneration || activeDocument !== document) return
    const target = canvas.value
    if (!target) return
    const width = Math.max(200, container.value.clientWidth - 32)
    const viewport = page.getViewport({ scale: width / page.getViewport({ scale: 1 }).width * zoom.value })
    const ratio = Math.min(window.devicePixelRatio || 1, 2)
    target.width = Math.ceil(viewport.width * ratio)
    target.height = Math.ceil(viewport.height * ratio)
    target.style.width = `${viewport.width}px`
    target.style.height = `${viewport.height}px`
    const task = page.render({ canvas: target, viewport, transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0] })
    renderTask = task
    await task.promise
  } catch (cause) {
    if (current === renderGeneration && !(cause instanceof Error && cause.name === 'RenderingCancelledException')) error.value = '本页暂时无法显示，请重新加载报告。'
  } finally { if (current === renderGeneration) rendering.value = false }
}
watch(() => props.url, load, { immediate: true })
watch([pageNumber, zoom], () => { if (!loading.value) void render() })
onMounted(() => { resizeObserver = new ResizeObserver(() => { if (!loading.value && document) void render() }); if (container.value) resizeObserver.observe(container.value) })
onBeforeUnmount(() => { loadGeneration++; renderGeneration++; resizeObserver?.disconnect(); renderTask?.cancel(); void loadingTask?.destroy().catch(() => {}) })
</script>

<style scoped>
.pdf-reader { min-height: 65vh; }
.reader-controls { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-tight); padding: 12px var(--space-content); color: var(--text-main); font-size: var(--font-small); background: var(--surface-subtle); }
.zoom-controls { display: flex; align-items: center; gap: var(--space-tight); margin-left: auto; }
.page-viewport { overflow: auto; max-height: 70vh; padding: var(--space-content); background: var(--surface-sunken); }
canvas { display: block; margin: auto; background: var(--surface); }
.reader-note { text-align: center; color: var(--text-muted); font-size: var(--font-small); }
</style>
