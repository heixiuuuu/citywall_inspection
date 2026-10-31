<template>
  <div class="page">
    <div class="toolbar">
      <el-checkbox v-model="pendingAdmission" @change="search">仅待受理</el-checkbox>
      <el-select v-model="filter.status" placeholder="审核状态" clearable style="width: 130px">
        <el-option label="草稿" value="DRAFT" />
        <el-option label="待审核" value="SUBMITTED" />
        <el-option label="已退回" value="RETURNED" />
        <el-option label="已通过" value="ACCEPTED" />
        <el-option label="已驳回" value="REJECTED" />
      </el-select>
      <el-select v-model="filter.source" placeholder="来源" clearable style="width: 120px">
        <el-option label="任务巡检" value="TASK" />
        <el-option label="主动上报" value="VOLUNTARY" />
      </el-select>
      <el-input-number v-model="filter.task_id" :min="1" placeholder="任务 ID" controls-position="right" style="width: 130px" />
      <el-input-number v-model="filter.point_id" :min="1" placeholder="点位 ID" controls-position="right" style="width: 130px" />
      <el-date-picker v-model="dateRange" type="datetimerange" value-format="YYYY-MM-DDTHH:mm:ssZ" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" style="width: 360px" />
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="reset">重置</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column label="来源" width="100">
        <template #default="{ row }">
          <el-tag :type="row.source === 'TASK' ? 'primary' : 'warning'" size="small">
            {{ row.source === 'TASK' ? '任务巡检' : '主动上报' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="受理" width="100"><template #default="{ row }">{{ row.admitted_at ? (row.no_issue_reason ? '不立项' : '已受理') : row.status === 'ACCEPTED' ? '待受理' : '—' }}</template></el-table-column>
      <el-table-column prop="reporter_id" label="上报人" width="80" />
      <el-table-column label="点位" width="110">
        <template #default="{ row }">{{ row.task_point_id ?? (row.point_id ?? '—') }}</template>
      </el-table-column>
      <el-table-column label="位置" min-width="170" show-overflow-tooltip>
        <template #default="{ row }">{{ row.location_text || `${row.latitude?.toFixed(5) ?? ''},${row.longitude?.toFixed(5) ?? ''}` }}</template>
      </el-table-column>
      <el-table-column label="结论" width="100">
        <template #default="{ row }">{{ outcomeLabel(row.inspection_outcome) }}</template>
      </el-table-column>
      <el-table-column label="提交时间" width="170">
        <template #default="{ row }">{{ formatDateTime(row.submitted_at || row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">查看</el-button>
          <el-button v-if="row.status === 'ACCEPTED' && !row.admitted_at" link type="primary" @click="openAdmission(row.id)">受理</el-button>
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

    <!-- 记录详情：原图 + 现场信息 -->
    <el-drawer v-model="detailVisible" title="现场记录详情" size="min(900px, 95vw)" destroy-on-close>
      <div v-if="detail">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="记录 ID">{{ detail.id }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ detail.source === 'TASK' ? '任务巡检' : '主动上报' }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ statusText(detail.status) }}</el-descriptions-item>
          <el-descriptions-item label="位置">{{ detail.location_text || '—' }}</el-descriptions-item>
          <el-descriptions-item label="坐标">
            {{ detail.latitude != null && detail.longitude != null ? `${detail.latitude.toFixed(6)}, ${detail.longitude.toFixed(6)}` : '—' }}
          </el-descriptions-item>
          <el-descriptions-item label="定位精度">{{ detail.accuracy_m != null ? `${detail.accuracy_m} m` : '—' }}</el-descriptions-item>
          <el-descriptions-item label="现场结论">{{ outcomeLabel(detail.inspection_outcome) }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ detail.revision }}</el-descriptions-item>
        </el-descriptions>
        <ObservationEvidence v-if="detailVisible" :observation-id="detail.id" />
      </div>
    </el-drawer>
    <IssueAdmission v-model="admitVisible" :observation-id="admitRecordId" @admitted="load" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { outcomeLabel, observationStatusLabel as statusText } from '@/shared/ui/labels'
import IssueAdmission from '@/shared/ui/IssueAdmission.vue'
import ObservationEvidence from '@/shared/ui/ObservationEvidence.vue'
import { formatDateTime } from '@/shared/ui/time'
import { ElMessage } from 'element-plus'
import { fetchObservations, type Observation } from '@/shared/api/observation'

const rows = ref<Observation[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const route = useRoute()
const pendingAdmission = ref(route.query.pending_admission === '1')

const filter = reactive<{ status: string; source: string; task_id: number | undefined; point_id: number | undefined }>({
  status: typeof route.query.status === 'string' ? route.query.status : '',
  source: '',
  task_id: undefined,
  point_id: undefined,
})
const dateRange = ref<[string, string] | null>(null)

const admitVisible = ref(false)
const admitRecordId = ref(0)
function openAdmission(id: number) { admitRecordId.value = id; admitVisible.value = true }
const detailVisible = ref(false)
const detail = ref<Observation | null>(null)

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await fetchObservations({
      pending_admission: pendingAdmission.value,
      status: filter.status || undefined,
      source: filter.source || undefined,
      task_id: filter.task_id,
      point_id: filter.point_id,
      date_from: dateRange.value?.[0] || undefined,
      date_to: dateRange.value?.[1] || undefined,
      page: page.value,
      page_size: pageSize,
    })
    rows.value = res.items
    total.value = res.total
  } catch {
    ElMessage.error('加载记录失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function reset() {
  pendingAdmission.value = false
  filter.status = ''
  filter.source = ''
  filter.task_id = undefined
  filter.point_id = undefined
  dateRange.value = null
  search()
}

function statusType(s: string) {
  return ({ DRAFT: 'info', SUBMITTED: 'danger', RETURNED: 'warning', ACCEPTED: 'success', REJECTED: 'info' }[s] ?? 'info') as 'info' | 'danger' | 'warning' | 'success'
}

async function openDetail(row: Observation) {
  detail.value = row
  detailVisible.value = true

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
  flex-wrap: wrap;
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
.photos {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.photo {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: #666;
}
.photo img {
  width: 150px;
  height: auto;
  border-radius: 8px;
  border: 1px solid #eee;
}
</style>
