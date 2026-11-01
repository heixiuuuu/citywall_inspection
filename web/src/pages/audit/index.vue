<template>
  <div class="audit-page">
    <PageHeading title="审计日志" description="查看关键业务操作、状态变化与处理说明，追溯巡检流程。" />
    <el-card shadow="never">
      <el-alert v-if="loadError && rows.length" :title="loadError" description="当前保留上次加载的日志。" type="error" :closable="false" class="load-notice" />
      <div class="filters">
        <el-select v-model="filters.entity_type" clearable placeholder="实体类型" style="width: 150px" @change="reload">
          <el-option v-for="(label, value) in entities" :key="value" :label="label" :value="value" />
        </el-select>
        <el-input v-model="filters.entity_id" placeholder="业务编号" inputmode="numeric" style="width: 140px" />
        <el-input v-model="filters.actor_id" placeholder="操作人编号" inputmode="numeric" style="width: 140px" />
        <el-button type="primary" @click="reload">查询</el-button>
        <el-button @click="reset">重置</el-button>
      </div>

      <el-table :data="rows" v-loading="loading" stripe>
        <template #empty><ListFeedback :error="loadError" title="没有找到对应操作记录" description="可调整业务类型、业务编号或操作人编号后重新查询。" icon="audit" @retry="load" /></template>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="业务类型" width="110"><template #default="{ row }">{{ entities[row.entity_type] || row.entity_type }}</template></el-table-column>
        <el-table-column prop="entity_id" label="业务编号" width="90" />
        <el-table-column label="操作" min-width="130"><template #default="{ row }">{{ actionLabel(row.action) }}</template></el-table-column>
        <el-table-column prop="actor_id" label="操作人" width="80" />
        <el-table-column label="状态变化" min-width="150">
          <template #default="{ row }">
            <span v-if="row.old_status || row.new_status">
              {{ statusLabel(row.entity_type, row.old_status) }} → {{ statusLabel(row.entity_type, row.new_status) }}
            </span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因/说明" min-width="180" show-overflow-tooltip />
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next, sizes"
          :total="total"
          :page-size="filters.page_size ?? 50"
          :current-page="filters.page ?? 1"
          :page-sizes="[20, 50, 100]"
          @current-change="(p: number) => go(filters.page_size ?? 50, p)"
          @size-change="(s: number) => go(s, 1)"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchAuditEvents, type AuditEvent } from '@/shared/api/audit'
import PageHeading from '@/shared/ui/PageHeading.vue'
import ListFeedback from '@/shared/ui/ListFeedback.vue'
import { formatDateTime } from '@/shared/ui/time'
import { observationStatusLabel } from '@/shared/ui/labels'
import { ACTION_TYPE_TEXT, ISSUE_STATUS } from '@/shared/api/issue'

const loading = ref(false)
const loadError = ref('')
const entities: Record<string, string> = { task: '巡检任务', observation: '现场记录', issue: '病害问题', report: '巡检报告', user: '用户登录' }
function actionLabel(action: string) {
  if (action.startsWith('action:')) return ACTION_TYPE_TEXT[action.slice(7)] ?? action
  return ({ dispatch: '派发任务', submit_task: '提交任务', complete: '完成任务', cancel: '取消任务', submit: '提交记录', review: '审核记录', admit: '受理记录', assign: '指派责任人', request_verify: '申请核验', reopen: '重新打开', add_action_image: '补充处置照片', admit_link: '关联已有问题', admit_create: '新建问题', VERIFY_PASS: '核验通过', VERIFY_RETURN: '核验退回', login: '账号登录', wechat_login: '微信登录', 'generate:TASK': '生成任务报告', 'generate:DATE_RANGE': '生成范围报告' } as Record<string, string>)[action] ?? action
}
function statusLabel(entity: string, value: string | null) {
  if (!value) return '—'
  if (entity === 'observation') return observationStatusLabel(value)
  if (entity === 'issue') return ISSUE_STATUS[value]?.label ?? value
  return ({ DRAFT: '草稿', ASSIGNED: '已派发', IN_PROGRESS: '进行中', PENDING_REVIEW: '待审核', RETURNED: '退回补正', COMPLETED: '已完成', CANCELLED: '已取消', GENERATING: '生成中', READY: '可查阅', FAILED: '生成失败' } as Record<string, string>)[value] ?? value
}
const rows = ref<AuditEvent[]>([])
const total = ref(0)
const filters = reactive<{ entity_type?: string; entity_id?: string; actor_id?: string; page?: number; page_size?: number }>({
  entity_type: undefined,
  entity_id: '',
  actor_id: '',
  page: 1,
  page_size: 50,
})

function formatTime(s: string): string {
  return formatDateTime(s)
}

async function load() {
  if (loading.value) return
  loading.value = true
  loadError.value = ''
  try {
    const res = await fetchAuditEvents({
      entity_type: filters.entity_type,
      entity_id: filters.entity_id || undefined,
      actor_id: filters.actor_id || undefined,
      page: filters.page,
      page_size: filters.page_size,
    })
    rows.value = res.events ?? []
    total.value = Number(res.total)
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '日志加载失败，请重新加载。'
  } finally {
    loading.value = false
  }
}

function reload() {
  if ([filters.entity_id, filters.actor_id].some(value => value && !/^[1-9]\d*$/.test(value))) { ElMessage.warning('业务编号和操作人编号须为正整数'); return }
  filters.page = 1
  load()
}

function go(pageSize: number, page: number) {
  filters.page_size = pageSize
  filters.page = page
  load()
}

function reset() {
  filters.entity_type = undefined
  filters.entity_id = ''
  filters.actor_id = ''
  filters.page = 1
  load()
}

onMounted(load)
</script>

<style scoped>
.audit-page :deep(.el-card) { border: none; border-radius: var(--radius-panel); }
.filters {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}
</style>
