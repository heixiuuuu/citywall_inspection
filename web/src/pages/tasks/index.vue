<template>
  <div class="page">
    <div class="toolbar">
      <el-button v-if="isAdmin" type="primary" @click="openCreate">新建任务</el-button>
      <el-button @click="load">刷新</el-button>
      <el-input v-model="keyword" placeholder="搜索任务名称" clearable style="width: 210px" @clear="search" />
      <el-button @click="search">查询</el-button>
      <el-checkbox v-model="overdueOnly" @change="search">仅超期任务</el-checkbox>
    </div>
    <div class="toolbar" aria-label="任务状态筛选">
      <el-check-tag :checked="!activeStatus" @change="activeStatus = ''">全部</el-check-tag>
      <el-check-tag
        v-for="s in statusFilter"
        :key="s.value"
        :checked="activeStatus === s.value"
        class="tag-filter"
        @change="activeStatus = activeStatus === s.value ? '' : s.value"
      >
        {{ s.label }}
      </el-check-tag>
    </div>

    <el-table :data="tasks" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="title" label="标题" min-width="180" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="负责人" min-width="130"><template #default="{row}">{{ assigneeName(row.assignee_id) }}</template></el-table-column>
      <el-table-column label="截止" width="120">
        <template #default="{ row }"><span :class="{ overdue: isOverdue(row) }">{{ row.due_at ? row.due_at.slice(0, 10) : '—' }}</span><el-tag v-if="isOverdue(row)" type="warning" size="small">已超期</el-tag></template>
      </el-table-column>
      <el-table-column prop="revision" label="版本" width="70" />
      <el-table-column label="操作" width="300" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          <el-button v-if="isAdmin && row.status === 'DRAFT'" link type="warning" @click="dispatch(row)">派发</el-button>
          <el-button v-if="canReview && row.status === 'PENDING_REVIEW'" link type="success" @click="complete(row)">完成</el-button>
          <el-button
            v-if="isAdmin && ['DRAFT', 'ASSIGNED', 'IN_PROGRESS'].includes(row.status)"
            link
            type="danger"
            @click="cancel(row)"
          >取消</el-button>
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

    <!-- 创建任务 -->
    <el-dialog v-model="createVisible" title="新建巡检任务" width="520" :close-on-click-modal="false">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="标题" required>
          <el-input v-model="createForm.title" maxlength="128" placeholder="如：东城墙 2 号段巡检" />
        </el-form-item>
        <el-form-item label="巡检员" required>
          <el-select v-model="createForm.assignee_id" filterable placeholder="选择有效巡检员" style="width:100%">
            <el-option v-for="user in assignees" :key="user.id" :value="user.id" :label="`${user.display_name}（#${user.id}）`" />
          </el-select>
        </el-form-item>
        <el-form-item label="截止时间">
          <el-date-picker v-model="createForm.due_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="createForm.description" type="textarea" :rows="2" maxlength="512" show-word-limit />
        </el-form-item>
        <el-form-item label="点位" required>
          <el-select v-model="createForm.point_ids" multiple filterable placeholder="选择点位" style="width: 100%">
            <el-option v-for="p in points" :key="p.id" :label="`${p.code} ${p.name}`" :value="p.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="create">创建</el-button>
      </template>
    </el-dialog>

    <!-- 任务详情 -->
    <el-drawer v-model="detailVisible" title="任务详情" size="min(1000px, 95vw)" destroy-on-close>
      <div v-if="detail">
        <div v-if="canReview" class="toolbar"><el-button @click="$router.push({ path: '/records', query: { task_id: detail.task.id } })">查看现场记录</el-button><el-button type="primary" @click="$router.push({ path: '/reports', query: { task_id: detail.task.id } })">生成任务报告</el-button></div>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="标题">{{ detail.task.title }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusType(detail.task.status)" size="small">{{ statusText(detail.task.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="负责人">{{ assigneeName(detail.task.assignee_id) }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ detail.task.revision }}</el-descriptions-item>
          <el-descriptions-item label="截止">{{ formatDateTime(detail.task.due_at) }}</el-descriptions-item>
          <el-descriptions-item label="说明">{{ detail.task.description || '—' }}</el-descriptions-item>
        </el-descriptions>
        <h4 class="subtitle">点位列表</h4>
        <div class="progress-summary"><span>已提交 <strong>{{ submittedPointCount }}</strong> / {{ detail.points.length }}</span><span>审核通过 <strong>{{ acceptedPointCount }}</strong></span><span>退回补正 <strong>{{ returnedPointCount }}</strong></span></div>
        <el-progress :percentage="detail.points.length ? Math.round(submittedPointCount / detail.points.length * 100) : 0" :show-text="false" />
        <el-table :data="detail.points" row-key="id" size="small" stripe class="point-table">
          <el-table-column type="expand"><template #default="{ row }"><ObservationEvidence v-if="row.observation_id && (canReview || row.observation_reporter_id === myId)" :observation-id="row.observation_id" /><el-empty v-else :description="row.observation_id ? '该记录由其他人员采集，请联系管理员查看' : '该点位尚未创建现场记录'" :image-size="56" /></template></el-table-column>
          <el-table-column prop="sequence_no" label="#" width="56" />
          <el-table-column label="点位名称" min-width="140"><template #default="{ row }">{{ row.snapshot_name || row.point_name || `点位 #${row.point_id}` }}</template></el-table-column>
          <el-table-column label="记录状态" min-width="105"><template #default="{ row }"><el-tag v-if="row.observation_id" :type="pointStatusType(row.observation_status)" size="small">{{ observationStatusLabel(row.observation_status) }}</el-tag><span v-else class="muted">待采集</span></template></el-table-column>
          <el-table-column label="现场结论" min-width="105"><template #default="{ row }">{{ outcomeLabel(row.observation_outcome) }}</template></el-table-column>
          <el-table-column prop="snapshot_location_hint" label="位置提示" min-width="140" />
          <el-table-column label="坐标" min-width="160">
            <template #default="{ row }">
              {{ row.snapshot_latitude?.toFixed(5) }}, {{ row.snapshot_longitude?.toFixed(5) }}
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { formatDateTime } from '@/shared/ui/time'
import { observationStatusLabel, outcomeLabel } from '@/shared/ui/labels'
import ObservationEvidence from '@/shared/ui/ObservationEvidence.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchTasks, createTask, fetchTask, dispatchTask, completeTask, cancelTask, fetchAssignees, type Assignee } from '@/shared/api/task'
import { fetchPoints, type InspectionPoint } from '@/shared/api/site'

const roles: string[] = JSON.parse(localStorage.getItem('city_wall_roles') ?? '[]')
const isAdmin = computed(() => roles.includes('ADMIN'))
const canReview = computed(() => roles.includes('REVIEWER') || roles.includes('ADMIN'))
const myId = Number(JSON.parse(localStorage.getItem('city_wall_user') ?? 'null')?.id ?? 0)

const statusFilter = [
  { value: 'DRAFT', label: '草稿', type: 'info' },
  { value: 'ASSIGNED', label: '已派发', type: 'primary' },
  { value: 'IN_PROGRESS', label: '采集中', type: 'warning' },
  { value: 'RETURNED', label: '退回补正', type: 'warning' },
  { value: 'PENDING_REVIEW', label: '待审核', type: 'danger' },
  { value: 'COMPLETED', label: '已完成', type: 'success' },
  { value: 'CANCELLED', label: '已取消', type: 'info' },
] as const

const tasks = ref<any[]>([])
const points = ref<InspectionPoint[]>([])
const assignees = ref<Assignee[]>([])
const loading = ref(false)
const route = useRoute()
const activeStatus = ref(typeof route.query.status === 'string' && statusFilter.some(s => s.value === route.query.status) ? route.query.status : '')
const keyword = ref('')
const overdueOnly = ref(route.query.overdue === '1')
function search() { page.value = 1; void load() }
function isOverdue(row: { due_at: string | null; status: string }) {
  return !!row.due_at && ['ASSIGNED', 'IN_PROGRESS', 'RETURNED', 'PENDING_REVIEW'].includes(row.status) && Date.parse(row.due_at) < Date.now()
}
const page = ref(1)
const pageSize = 20
const total = ref(0)

// 切换状态过滤时回到第一页并从服务端重新拉取。
watch(activeStatus, () => {
  page.value = 1
  load()
})

const createVisible = ref(false)
const creating = ref(false)
const createForm = reactive<{ title: string; assignee_id: number | undefined; due_at: string | null; description: string; point_ids: number[] }>({
  title: '',
  assignee_id: undefined,
  due_at: null,
  description: '',
  point_ids: [],
})

const detailVisible = ref(false)
const detail = ref<any>(null)
const submittedPointCount = computed(() => detail.value?.points.filter((point: { observation_status: string }) => ['SUBMITTED', 'ACCEPTED'].includes(point.observation_status)).length ?? 0)
const acceptedPointCount = computed(() => detail.value?.points.filter((point: { observation_status: string }) => point.observation_status === 'ACCEPTED').length ?? 0)
const returnedPointCount = computed(() => detail.value?.points.filter((point: { observation_status: string }) => point.observation_status === 'RETURNED').length ?? 0)
function pointStatusType(status: string) {
  const types = { DRAFT: 'info', SUBMITTED: 'primary', RETURNED: 'warning', ACCEPTED: 'success', REJECTED: 'danger' } as const
  return status in types ? types[status as keyof typeof types] : 'info'
}

onMounted(() => {
  load()
  if (isAdmin.value) loadPoints()
  if (canReview.value) loadAssignees()
})

async function loadAssignees() {
  try { assignees.value = await fetchAssignees() }
  catch { ElMessage.error('加载巡检员失败') }
}
function assigneeName(id: number) { return assignees.value.find(user => user.id === id)?.display_name || `#${id}` }

async function load() {
  loading.value = true
  try {
    const res = await fetchTasks({
      keyword: keyword.value.trim() || undefined,
      overdue: overdueOnly.value,
      status: activeStatus.value || undefined,
      page: page.value,
      page_size: pageSize,
    })
    tasks.value = res.tasks
    total.value = res.total
  } catch {
    ElMessage.error('加载任务失败')
  } finally {
    loading.value = false
  }
}

async function loadPoints() {
  try {
    points.value = await fetchPoints()
  } catch {
    ElMessage.error('加载点位失败')
  }
}

function statusText(s: string) {
  return (
    {
      DRAFT: '草稿',
      ASSIGNED: '已派发',
      IN_PROGRESS: '采集中',
      PENDING_REVIEW: '待审核',
      RETURNED: '已退回',
      COMPLETED: '已完成',
      CANCELLED: '已取消',
    }[s] ?? s
  )
}

function statusType(s: string) {
  return (
    {
      DRAFT: 'info',
      ASSIGNED: 'primary',
      IN_PROGRESS: 'warning',
      PENDING_REVIEW: 'danger',
      RETURNED: 'danger',
      COMPLETED: 'success',
      CANCELLED: 'info',
    }[s] ?? 'info'
  ) as 'info' | 'primary' | 'warning' | 'danger' | 'success'
}

function openCreate() {
  Object.assign(createForm, { title: '', assignee_id: undefined, due_at: null, description: '', point_ids: [] })
  loadAssignees()
  loadPoints()
  createVisible.value = true
}

async function create() {
  if (creating.value) return
  if (!createForm.title.trim() || !createForm.assignee_id || !createForm.point_ids.length) {
    ElMessage.warning('请填写标题并选择巡检员和点位')
    return
  }
  creating.value = true
  try {
    await createTask({
      title: createForm.title.trim(),
      assignee_id: createForm.assignee_id,
      due_at: createForm.due_at ?? undefined,
      description: createForm.description || undefined,
      point_ids: createForm.point_ids,
    })
    ElMessage.success('任务已创建（草稿）')
    createVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
  } finally {
    creating.value = false
  }
}

async function openDetail(row: any) {
  try {
    detail.value = await fetchTask(row.id)
    detailVisible.value = true
  } catch {
    ElMessage.error('加载详情失败')
  }
}

async function dispatch(row: any) {
  try {
    await ElMessageBox.confirm(`确认派发任务「${row.title}」？`, '派发确认', { type: 'warning' })
    const t = await dispatchTask(row.id, row.revision)
    ElMessage.success(`已派发（版本 ${t.revision}）`)
    load()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || '派发失败')
  }
}

async function complete(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      '填写完成说明（将写入任务记录，供审计追溯）',
      `确认完成「${row.title}」？`,
      {
        type: 'warning',
        inputType: 'textarea',
        inputPlaceholder: '如：全部点位已巡检，3 处疑似病害已提交审核',
        inputValidator: (v) => (v ? true : '完成说明必填'),
      },
    )
    const t = await completeTask(row.id, row.revision, value)
    ElMessage.success(`任务已完成（版本 ${t.revision}）`)
    load()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || '操作失败')
  }
}

async function cancel(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      '填写取消原因（将写入任务与审计记录）',
      `确认取消「${row.title}」？`,
      {
        type: 'warning',
        inputType: 'textarea',
        inputPlaceholder: '如：巡检计划调整，改期执行',
        inputValidator: (v) => (v ? true : '取消原因必填'),
      },
    )
    const t = await cancelTask(row.id, row.revision, value)
    ElMessage.success(`任务已取消（版本 ${t.revision}）`)
    load()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || '操作失败')
  }
}
</script>

<style scoped>
.progress-summary { display: flex; gap: var(--space-section); flex-wrap: wrap; margin-bottom: var(--space-content); font-size: 13px; color: var(--text-muted); }
.progress-summary strong { color: var(--brand); font-weight: 600; }
.point-table { margin-top: var(--space-content); }
.muted { color: var(--text-muted); }
.overdue { color: var(--el-color-warning); display: block; }
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
.tag-filter {
  cursor: pointer;
}
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
.subtitle {
  margin: 20px 0 10px;
  color: #2b2b2b;
}
</style>
