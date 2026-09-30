<template>
  <div class="page">
    <div class="toolbar">
      <el-button v-if="isAdmin" type="primary" @click="openCreate">新建任务</el-button>
      <el-button @click="load">刷新</el-button>
      <el-tag
        v-for="s in statusFilter"
        :key="s.value"
        :type="s.type"
        size="small"
        class="tag-filter"
        @click="activeStatus = activeStatus === s.value ? '' : s.value"
      >
        {{ s.label }}
      </el-tag>
    </div>

    <el-table :data="filtered" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="title" label="标题" min-width="180" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="assignee_id" label="负责人ID" width="100" />
      <el-table-column label="截止" width="120">
        <template #default="{ row }">{{ row.due_at ? row.due_at.slice(0, 10) : '—' }}</template>
      </el-table-column>
      <el-table-column prop="revision" label="版本" width="70" />
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          <el-button v-if="isAdmin && row.status === 'DRAFT'" link type="warning" @click="dispatch(row)">派发</el-button>
          <el-button v-if="canReview && row.status === 'PENDING_REVIEW'" link type="success" @click="complete(row)">完成</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 创建任务 -->
    <el-dialog v-model="createVisible" title="新建巡检任务" width="520">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="标题" required>
          <el-input v-model="createForm.title" placeholder="如：东城墙 2 号段巡检" />
        </el-form-item>
        <el-form-item label="负责人ID" required>
          <el-input-number v-model="createForm.assignee_id" :min="1" />
        </el-form-item>
        <el-form-item label="截止时间">
          <el-date-picker v-model="createForm.due_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="createForm.description" type="textarea" :rows="2" />
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
    <el-drawer v-model="detailVisible" title="任务详情" size="560">
      <div v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="标题">{{ detail.task.title }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusType(detail.task.status)" size="small">{{ statusText(detail.task.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="负责人ID">{{ detail.task.assignee_id }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ detail.task.revision }}</el-descriptions-item>
          <el-descriptions-item label="截止">{{ detail.task.due_at || '—' }}</el-descriptions-item>
          <el-descriptions-item label="说明">{{ detail.task.description || '—' }}</el-descriptions-item>
        </el-descriptions>
        <h4 class="subtitle">点位列表</h4>
        <el-table :data="detail.points" size="small" stripe>
          <el-table-column prop="sequence_no" label="#" width="56" />
          <el-table-column prop="snapshot_name" label="快照名称" min-width="140" />
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
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchTasks, createTask, fetchTask, dispatchTask, completeTask } from '@/shared/api/task'
import { fetchPoints, type InspectionPoint } from '@/shared/api/site'

const roles: string[] = JSON.parse(localStorage.getItem('city_wall_roles') ?? '[]')
const isAdmin = computed(() => roles.includes('ADMIN'))
const canReview = computed(() => roles.includes('REVIEWER') || roles.includes('ADMIN'))

const statusFilter = [
  { value: 'DRAFT', label: '草稿', type: 'info' },
  { value: 'ASSIGNED', label: '已派发', type: 'primary' },
  { value: 'IN_PROGRESS', label: '采集中', type: 'warning' },
  { value: 'PENDING_REVIEW', label: '待审核', type: 'danger' },
  { value: 'COMPLETED', label: '已完成', type: 'success' },
] as const

const tasks = ref<any[]>([])
const points = ref<InspectionPoint[]>([])
const loading = ref(false)
const activeStatus = ref('')

const filtered = computed(() =>
  activeStatus.value ? tasks.value.filter((t) => t.status === activeStatus.value) : tasks.value,
)

const createVisible = ref(false)
const creating = ref(false)
const createForm = reactive<{ title: string; assignee_id: number; due_at: string | null; description: string; point_ids: number[] }>({
  title: '',
  assignee_id: 1,
  due_at: null,
  description: '',
  point_ids: [],
})

const detailVisible = ref(false)
const detail = ref<any>(null)

onMounted(() => {
  load()
  if (isAdmin.value) loadPoints()
})

async function load() {
  loading.value = true
  try {
    tasks.value = await fetchTasks()
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
  createVisible.value = true
}

async function create() {
  if (!createForm.title || !createForm.point_ids.length) {
    ElMessage.warning('标题与点位必填')
    return
  }
  creating.value = true
  try {
    await createTask({
      title: createForm.title,
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
.tag-filter {
  cursor: pointer;
}
.subtitle {
  margin: 20px 0 10px;
  color: #2b2b2b;
}
</style>
