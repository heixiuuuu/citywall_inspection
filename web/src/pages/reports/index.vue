<template>
  <div class="page">
    <div class="toolbar">
      <el-button type="primary" @click="openTaskReport">生成任务报告</el-button>
      <el-button type="primary" plain @click="openRangeReport">生成范围报告</el-button>
      <el-button @click="load">刷新</el-button>
      <span class="hint">报告为生成时刻数据快照，后续问题状态变化不影响旧报告；生成失败可重试，不影响任务状态。</span>
    </div>

    <el-table :data="reports" v-loading="loading" stripe>
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
        <template #default="{ row }">{{ (row.created_at || '').slice(0, 19).replace('T', ' ') }}</template>
      </el-table-column>
      <el-table-column label="失败原因" min-width="140">
        <template #default="{ row }">{{ row.error_code || '—' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 'READY'" link type="primary" @click="download(row)">下载 PDF</el-button>
          <el-button v-if="row.status === 'FAILED'" link type="warning" @click="retry(row)">重试</el-button>
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

    <!-- 生成任务报告 -->
    <el-dialog v-model="taskVisible" title="生成任务报告" width="420">
      <el-form label-width="110px">
        <el-form-item label="任务 ID" required>
          <el-input-number v-model="taskId" :min="1" style="width: 100%" />
        </el-form-item>
        <p class="tip">报告将包含任务下所有点位：正常点、疑似病害点、批准未覆盖点、照片、人工与模型结论差异、审核意见与关联问题状态。</p>
      </el-form>
      <template #footer>
        <el-button @click="taskVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createTask">生成</el-button>
      </template>
    </el-dialog>

    <!-- 生成范围报告 -->
    <el-dialog v-model="rangeVisible" title="生成时间范围报告" width="420">
      <el-form label-width="110px">
        <el-form-item label="开始时间" required>
          <el-date-picker v-model="rangeStart" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width: 100%" />
        </el-form-item>
        <el-form-item label="结束时间" required>
          <el-date-picker v-model="rangeEnd" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rangeVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createRange">生成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchReports, createTaskReport, createRangeReport, retryReport, loadReportBlob, type Report } from '@/shared/api/report'

const reports = ref<Report[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)

const taskVisible = ref(false)
const rangeVisible = ref(false)
const creating = ref(false)
const taskId = ref(1)
const rangeStart = ref('')
const rangeEnd = ref('')

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await fetchReports({ page: page.value, page_size: pageSize })
    reports.value = res.items
    total.value = res.total
  } catch {
    ElMessage.error('加载报告失败')
  } finally {
    loading.value = false
  }
}

function statusText(s: string) {
  return { GENERATING: '生成中', READY: '就绪', FAILED: '失败' }[s] ?? s
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
  creating.value = true
  try {
    const r = await createTaskReport(taskId.value)
    ElMessage.success(`报告已生成（#${r.id}，状态 ${r.status}）`)
    taskVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '生成失败')
  } finally {
    creating.value = false
  }
}

async function createRange() {
  if (!rangeStart.value || !rangeEnd.value) {
    ElMessage.warning('起止时间必填')
    return
  }
  creating.value = true
  try {
    const r = await createRangeReport(rangeStart.value, rangeEnd.value)
    ElMessage.success(`范围报告已生成（#${r.id}，状态 ${r.status}）`)
    rangeVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '生成失败')
  } finally {
    creating.value = false
  }
}

async function retry(row: Report) {
  try {
    const r = await retryReport(row.id)
    ElMessage.success(`已重新生成（#${r.id}，状态 ${r.status}）`)
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '重试失败')
  }
}

async function download(row: Report) {
  try {
    const { blobUrl, fileName } = await loadReportBlob(row.id)
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = fileName
    a.click()
    setTimeout(() => URL.revokeObjectURL(blobUrl), 10_000)
  } catch (e: any) {
    ElMessage.error(e?.message || '下载失败')
  }
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
  margin-bottom: 14px;
}
.hint {
  color: #999;
  font-size: 12px;
  margin-left: auto;
}
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
