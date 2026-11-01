<template>
  <div class="page">
    <PageHeading title="报告中心" description="把巡检证据整理成报告，在线查阅后下载归档。" />
    <el-alert v-if="loadError && reports.length" :title="loadError" description="当前保留上次加载的报告列表。" type="error" :closable="false" class="load-notice" />
    <div class="toolbar">
      <el-button type="primary" @click="openTaskReport">生成任务报告</el-button>
      <el-button type="primary" plain @click="openRangeReport">生成范围报告</el-button>
      <el-button :loading="loading" @click="load">刷新列表</el-button>
    </div>

    <el-table :data="reports" v-loading="loading" stripe>
      <template #empty><ListFeedback :error="loadError" title="还没有巡检报告" description="选择巡检任务或时间范围，即可生成包含现场照片、复核意见和处置进展的报告。" icon="reports" @retry="load" /></template>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column label="范围" width="150">
        <template #default="{ row }">
          <el-tag :type="row.scope_type === 'TASK' ? 'primary' : 'warning'" size="small">
            {{ row.scope_type === 'TASK' ? `任务 #${row.task_id}` : '时间范围' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="文件" min-width="150">
        <template #default="{ row }">{{ row.status === 'READY' ? `report-${row.id}.pdf` : '—' }}</template>
      </el-table-column>
      <el-table-column label="生成时间" width="170">
        <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="失败原因" min-width="140">
        <template #default="{ row }">{{ row.status === 'FAILED' ? '生成未完成，可点击重新生成' : '—' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 'READY'" link type="primary" @click="openPreview(row)">在线预览</el-button>
          <el-button v-if="row.status === 'READY'" link type="primary" :loading="downloading.has(row.id)" @click="download(row)">下载 PDF</el-button>
          <el-button v-if="row.status === 'FAILED'" link type="warning" :loading="retrying.has(row.id)" @click="retry(row)">重新生成</el-button>
          <el-button v-if="row.status === 'GENERATING'" link disabled>生成中…</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="(p: number) => { page = p; load() }"
      />
    </div>
    <p class="snapshot-note">报告保留生成时刻的证据与处置进展。后续数据变化后，可重新生成一份报告。</p>

    <!-- 生成任务报告 -->
    <el-dialog v-model="taskVisible" title="生成任务报告" width="520" :close-on-click-modal="false" :close-on-press-escape="!creating" :show-close="!creating">
      <el-form label-width="110px" :disabled="creating">
        <el-form-item label="巡检任务" required>
          <TaskSelect v-model="taskId" />
        </el-form-item>
        <p class="tip">报告将包含任务下所有点位：正常点、疑似病害点、批准未覆盖点、照片、人工与模型结论差异、审核意见与关联问题状态。</p>
      </el-form>
      <template #footer>
        <el-button :disabled="creating" @click="taskVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createTask">{{ creating ? '正在整理报告…' : '生成报告' }}</el-button>
      </template>
    </el-dialog>

    <!-- 生成范围报告 -->
    <el-dialog v-model="rangeVisible" title="生成时间范围报告" width="520" :close-on-click-modal="false" :close-on-press-escape="!creating" :show-close="!creating">
      <el-form label-width="110px" :disabled="creating">
        <el-form-item label="开始时间" required>
          <el-date-picker v-model="rangeStart" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width: 100%" />
        </el-form-item>
        <el-form-item label="结束时间" required>
          <el-date-picker v-model="rangeEnd" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :disabled="creating" @click="rangeVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createRange">{{ creating ? '正在整理报告…' : '生成报告' }}</el-button>
      </template>
    </el-dialog>
    <el-drawer v-model="previewVisible" :title="`巡检报告 #${previewReport?.id ?? ''}`" size="min(1100px, 96vw)" destroy-on-close @closed="releasePreview">
      <div class="preview-toolbar"><span class="hint">{{ previewReport?.scope_type === 'TASK' ? `任务 #${previewReport.task_id}` : '时间范围报告' }} · {{ formatDateTime(previewReport?.created_at) }}</span><el-button v-if="previewReport" :loading="downloading.has(previewReport.id)" @click="download(previewReport)">下载此报告</el-button></div>
      <div v-loading="previewLoading" class="pdf-preview"><ListFeedback v-if="previewError" :error="previewError" title="报告预览" description="" icon="reports" @retry="previewReport && openPreview(previewReport)" /><PdfPreview v-else-if="previewUrl" :url="previewUrl" /><p v-else class="hint">正在准备报告预览…</p></div>
      <p class="snapshot-note">可翻页或放大查阅；下载 PDF 可保留完整报告。</p>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { formatDateTime } from '@/shared/ui/time'
import { defineAsyncComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import TaskSelect from '@/shared/ui/TaskSelect.vue'
import PageHeading from '@/shared/ui/PageHeading.vue'
import ListFeedback from '@/shared/ui/ListFeedback.vue'
const PdfPreview = defineAsyncComponent({ loader: () => import('@/shared/ui/PdfPreview.vue'), loadingComponent: { render: () => h('p', { class: 'hint' }, '正在加载报告阅读器…') }, delay: 0 })
import { ElMessage } from 'element-plus'
import { fetchReports, createTaskReport, createRangeReport, retryReport, loadReportBlob, type Report } from '@/shared/api/report'

const reports = ref<Report[]>([])
const loading = ref(false)
const loadError = ref('')
const downloading = ref(new Set<number>())
const retrying = ref(new Set<number>())
const previewVisible = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
const previewReport = ref<Report | null>(null)
const previewUrl = ref('')
let previewGeneration = 0
const page = ref(1)
const pageSize = 20
const total = ref(0)

const taskVisible = ref(false)
const rangeVisible = ref(false)
const creating = ref(false)
const taskId = ref<number>()
const rangeStart = ref('')
const rangeEnd = ref('')

const route = useRoute()
onMounted(() => {
  void load()
  const initialTask = Number(route.query.task_id)
  if (Number.isSafeInteger(initialTask) && initialTask > 0) { taskId.value = initialTask; taskVisible.value = true }
})

async function load() {
  if (loading.value) return
  loading.value = true
  loadError.value = ''
  try {
    const res = await fetchReports({ page: page.value, page_size: pageSize })
    reports.value = res.items ?? []
    total.value = res.total
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '报告加载失败，请重新加载。'
  } finally {
    loading.value = false
  }
}

function statusText(s: string) {
  return { GENERATING: '正在生成', READY: '可查阅', FAILED: '生成失败' }[s] ?? s
}
function statusType(s: string) {
  return ({ GENERATING: 'warning', READY: 'success', FAILED: 'danger' }[s] ?? 'info') as 'warning' | 'success' | 'danger' | 'info'
}

function openTaskReport() {
  taskVisible.value = true
}
function openRangeReport() {
  rangeVisible.value = true
}

async function createTask() {
  if (creating.value) return
  if (!taskId.value) { ElMessage.warning('请选择巡检任务'); return }
  creating.value = true
  try {
    const r = await createTaskReport(taskId.value)
    ElMessage.success(`报告 #${r.id}：${statusText(r.status)}`)
    taskVisible.value = false
    load()
    if (r.status === 'READY') void openPreview(r)
  } catch (e: any) {
    ElMessage.error(e?.message || '生成失败')
  } finally {
    creating.value = false
  }
}

async function createRange() {
  if (creating.value) return
  if (!rangeStart.value || !rangeEnd.value) {
    ElMessage.warning('起止时间必填')
    return
  }
  if (Date.parse(rangeStart.value) >= Date.parse(rangeEnd.value)) { ElMessage.warning('结束时间须晚于开始时间'); return }
  creating.value = true
  try {
    const r = await createRangeReport(rangeStart.value, rangeEnd.value)
    ElMessage.success(`范围报告 #${r.id}：${statusText(r.status)}`)
    rangeVisible.value = false
    load()
    if (r.status === 'READY') void openPreview(r)
  } catch (e: any) {
    ElMessage.error(e?.message || '生成失败')
  } finally {
    creating.value = false
  }
}

async function retry(row: Report) {
  if (retrying.value.has(row.id)) return
  retrying.value.add(row.id)
  try {
    const r = await retryReport(row.id)
    ElMessage.success(`报告 #${r.id}：${statusText(r.status)}`)
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '重试失败')
  } finally { retrying.value.delete(row.id) }
}

async function download(row: Report) {
  if (downloading.value.has(row.id)) return
  downloading.value.add(row.id)
  try {
    const { blobUrl, fileName } = await loadReportBlob(row.id)
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = fileName
    a.click()
    ElMessage.success('已发起报告下载，请查看浏览器下载列表。')
    setTimeout(() => URL.revokeObjectURL(blobUrl), 10_000)
  } catch (e: any) {
    ElMessage.error(e?.message || '下载失败')
  } finally { downloading.value.delete(row.id) }
}

function releasePreview() { previewGeneration++; if (previewUrl.value) URL.revokeObjectURL(previewUrl.value); previewUrl.value = '' }
onBeforeUnmount(releasePreview)
async function openPreview(row: Report) {
  releasePreview()
  const current = previewGeneration
  previewReport.value = row
  previewVisible.value = true
  previewLoading.value = true
  previewError.value = ''
  try {
    const result = await loadReportBlob(row.id)
    if (current !== previewGeneration || !previewVisible.value) { URL.revokeObjectURL(result.blobUrl); return }
    previewUrl.value = result.blobUrl
  } catch (error) { if (current === previewGeneration) previewError.value = error instanceof Error ? error.message : '报告预览加载失败，请重试。' }
  finally { if (current === previewGeneration) previewLoading.value = false }
}
</script>

<style scoped>
.page {
  background: #fff;
  border-radius: 12px;
  padding: 16px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.hint {
  color: var(--text-muted);
  font-size: 12px;
  margin-left: auto;
}
.snapshot-note { color: var(--text-muted); font-size: var(--font-caption); margin: var(--space-content) 0 0; }
.preview-toolbar { display: flex; justify-content: space-between; align-items: center; gap: var(--space-content); margin-bottom: var(--space-content); }
.preview-toolbar .hint { margin: 0; }
.pdf-preview { min-height: 65vh; background: var(--surface-subtle); border-radius: var(--radius-control); overflow: hidden; }
.pdf-preview > p { padding: var(--space-section); }
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
.tip {
  color: #666;
  font-size: 12px;
  line-height: 1.6;
  margin: 0;
}
</style>
