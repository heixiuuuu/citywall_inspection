<template>
  <div class="page">
    <div class="toolbar">
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

    <el-table :data="issues" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="title" label="标题" min-width="180" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="ISSUE_STATUS[row.status]?.type" size="small">{{ ISSUE_STATUS[row.status]?.label ?? row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="优先级" width="100">
        <template #default="{ row }">
          <el-tag :type="ISSUE_PRIORITY[row.priority_code]?.type" size="small" effect="plain">
            {{ ISSUE_PRIORITY[row.priority_code]?.label ?? row.priority_code }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="location_text" label="位置" min-width="160" show-overflow-tooltip />
      <el-table-column prop="assignee_id" label="责任人ID" width="100" />
      <el-table-column label="开启" width="120">
        <template #default="{ row }">{{ formatDateTime(row.opened_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
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

    <!-- 问题详情：信息 + 关联记录 + 处置时间线 -->
    <el-drawer v-model="detailVisible" title="问题详情" size="620">
      <div v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="标题">{{ detail.issue.title }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="ISSUE_STATUS[detail.issue.status]?.type" size="small">{{ ISSUE_STATUS[detail.issue.status]?.label ?? detail.issue.status }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="优先级">{{ ISSUE_PRIORITY[detail.issue.priority_code]?.label ?? detail.issue.priority_code }}</el-descriptions-item>
          <el-descriptions-item label="责任人ID">{{ detail.issue.assignee_id ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="位置" :span="2">{{ detail.issue.location_text }}</el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">{{ detail.issue.description || '—' }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.priority_reason" label="优先级依据" :span="2">{{ detail.issue.priority_reason }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.closed_at" label="关闭时间">{{ formatDateTime(detail.issue.closed_at) }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.close_reason" label="关闭原因" :span="2">{{ detail.issue.close_reason }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ detail.issue.revision }}</el-descriptions-item>
        </el-descriptions>

        <h4 class="subtitle">关联现场记录</h4>
        <el-table :data="detail.observations" size="small" stripe>
          <el-table-column prop="observation_id" label="记录ID" width="90" />
          <el-table-column prop="source" label="来源" width="90">
            <template #default="{ row }">{{ row.source === 'VOLUNTARY' ? '主动上报' : row.source === 'TASK' ? '任务巡检' : '来源未知' }}</template>
          </el-table-column>
          <el-table-column prop="link_reason" label="关联原因" min-width="120" />
          <el-table-column prop="linked_at" label="关联时间" min-width="150">
            <template #default="{ row }">{{ formatDateTime(row.linked_at) }}</template>
          </el-table-column>
        </el-table>

        <h4 class="subtitle">处置时间线</h4>
        <el-timeline class="timeline">
          <el-timeline-item v-for="a in detail.actions" :key="a.id" :timestamp="formatDateTime(a.created_at)">
            <div class="action-line">
              <el-tag size="small" effect="plain">{{ ACTION_TYPE_TEXT[a.action_type] ?? a.action_type }}</el-tag>
              <span class="action-actor">操作人 #{{ a.actor_id }}</span>
            </div>
            <div class="action-content">{{ a.content }}</div>
          </el-timeline-item>
        </el-timeline>

        <!-- 处置操作区 -->
        <div class="ops">
          <el-button v-if="isAdmin && canAssign" type="primary" @click="openAssign">指派/改派</el-button>
          <el-button v-if="canAct && canAction" type="success" @click="openAction">提交进展/观察</el-button>
          <el-button v-if="canAct && canRequestVerify" type="warning" @click="openRequestVerify">申请核验</el-button>
          <el-button v-if="canVerify" type="danger" @click="openVerify">核验</el-button>
          <el-button v-if="canReview && isClosed" type="danger" @click="openReopen">重新打开</el-button>
          <span v-if="awaitingOtherReviewer" class="action-actor">需由另一名审核员核验</span>
        </div>
      </div>
    </el-drawer>

    <!-- 指派 -->
    <el-dialog v-model="assignVisible" title="指派/改派责任人" width="420">
      <el-form label-width="100px">
        <el-form-item label="责任人" required>
          <el-select v-model="assignForm.assignee_id" filterable :loading="assigneeLoading" :disabled="assigneeLoading || busy" placeholder="选择责任人" style="width: 100%">
            <el-option v-for="user in assignees" :key="user.id" :value="user.id" :label="`${user.display_name}（#${user.id}）`" />
          </el-select>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="assignForm.reason" placeholder="如：由工程科处理" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doAssign">确认</el-button>
      </template>
    </el-dialog>

    <!-- 处置动作 -->
    <el-dialog v-model="actionVisible" title="提交处置动作" width="480">
      <el-form label-width="100px">
        <el-form-item label="动作类型" required>
          <el-radio-group v-model="actionForm.action_type">
            <el-radio value="PROGRESS">处理进展</el-radio>
            <el-radio value="OBSERVE">观察记录</el-radio>
            <el-radio value="ADVICE">处理建议</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="actionForm.content" type="textarea" :rows="3" placeholder="描述进展、观察安排或建议" />
        </el-form-item>
        <el-form-item v-if="actionForm.action_type === 'OBSERVE'" label="复查时间">
          <el-date-picker v-model="actionForm.planned_follow_up_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="actionVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doAction">提交</el-button>
      </template>
    </el-dialog>

    <!-- 申请核验 -->
    <el-dialog v-model="verifyRequestVisible" title="申请核验" width="480">
      <el-form label-width="100px">
        <el-form-item label="核验说明" required>
          <el-input v-model="verifyRequestContent" type="textarea" :rows="3" placeholder="说明处理结论与证据情况" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="verifyRequestVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doRequestVerify">提交</el-button>
      </template>
    </el-dialog>

    <!-- 核验 -->
    <el-dialog v-model="verifyVisible" title="核验问题" width="480">
      <el-form label-width="100px">
        <el-form-item label="结论" required>
          <el-radio-group v-model="verifyForm.decision">
            <el-radio value="PASS">核验通过（关闭）</el-radio>
            <el-radio value="RETURN">退回继续处理</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="意见" required>
          <el-input v-model="verifyForm.reason" type="textarea" :rows="3" placeholder="退回必须填写具体原因" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="verifyVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doVerify">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { formatDateTime } from '@/shared/ui/time'
import {
  fetchIssues,
  fetchIssue,
  assignIssue,
  fetchIssueAssignees,
  type IssueAssignee,
  addIssueAction,
  requestVerification,
  verifyIssue,
  reopenIssue,
  ISSUE_STATUS,
  ISSUE_PRIORITY,
  ACTION_TYPE_TEXT,
  type Issue,
  type IssueAction,
  type IssueObservationLink,
} from '@/shared/api/issue'

const roles: string[] = JSON.parse(localStorage.getItem('city_wall_roles') ?? '[]')
const isAdmin = computed(() => roles.includes('ADMIN'))
const canReview = computed(() => roles.includes('REVIEWER') || roles.includes('ADMIN'))
const userObj = JSON.parse(localStorage.getItem('city_wall_user') ?? 'null')
const myId = Number(userObj?.id ?? 0)

const statusFilter = Object.entries(ISSUE_STATUS).map(([value, s]) => ({ value, ...s }))

const issues = ref<Issue[]>([])
const loading = ref(false)
const activeStatus = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)

watch(activeStatus, () => {
  page.value = 1
  load()
})

const detailVisible = ref(false)
const detail = ref<{
  issue: Issue
  observations: IssueObservationLink[]
  actions: IssueAction[]
} | null>(null)

const busy = ref(false)

const canAssign = computed(() => detail.value && ['UNASSIGNED', 'OPEN', 'IN_PROGRESS', 'MONITORING'].includes(detail.value.issue.status))
const canAction = computed(() => detail.value && ['OPEN', 'IN_PROGRESS', 'MONITORING'].includes(detail.value.issue.status))
const canRequestVerify = computed(() => detail.value && ['IN_PROGRESS', 'MONITORING'].includes(detail.value.issue.status))
const awaitingOtherReviewer = computed(() => detail.value?.issue.status === 'PENDING_VERIFICATION' && [...detail.value.actions].reverse().find(action => action.action_type === 'REQUEST_VERIFY')?.actor_id === myId)
const canVerify = computed(() => detail.value && canReview.value && detail.value.issue.status === 'PENDING_VERIFICATION' && !awaitingOtherReviewer.value)
const isClosed = computed(() => detail.value?.issue.status === 'CLOSED')
// 责任人本人或管理员/审核员可操作（与 Go 侧 canAct 一致）。
const canAct = computed(() => {
  if (!detail.value) return false
  return canReview.value || detail.value.issue.assignee_id === myId
})

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await fetchIssues({ status: activeStatus.value || undefined, page: page.value, page_size: pageSize })
    issues.value = res.items
    total.value = res.total
  } catch {
    ElMessage.error('加载问题台账失败')
  } finally {
    loading.value = false
  }
}

async function openDetail(row: Issue) {
  try {
    const d = await fetchIssue(row.id)
    detail.value = d
    detailVisible.value = true
  } catch {
    ElMessage.error('加载问题详情失败')
  }
}

// ---- 指派 ----
const assignVisible = ref(false)
const assignForm = reactive<{ assignee_id: number | undefined; reason: string }>({ assignee_id: undefined, reason: '' })
const assignees = ref<IssueAssignee[]>([])
const assigneeLoading = ref(false)
async function openAssign() {
  assignForm.assignee_id = undefined
  assignForm.reason = ''
  assignees.value = []
  assigneeLoading.value = true
  assignVisible.value = true
  try {
    assignees.value = await fetchIssueAssignees()
    const currentId = detail.value?.issue.assignee_id
    if (assignees.value.some(user => user.id === currentId)) assignForm.assignee_id = currentId ?? undefined
  } catch {
    ElMessage.error('加载责任人失败，请重试')
    assignVisible.value = false
  } finally { assigneeLoading.value = false }
}
async function doAssign() {
  if (!assignForm.assignee_id || assigneeLoading.value) { ElMessage.warning('请选择责任人'); return }
  const d = detail.value!
  busy.value = true
  try {
    await assignIssue(d.issue.id, d.issue.revision, assignForm.assignee_id, assignForm.reason)
    ElMessage.success('已指派')
    assignVisible.value = false
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '指派失败')
  } finally {
    busy.value = false
  }
}

// ---- 处置动作 ----
const actionVisible = ref(false)
const actionForm = reactive<{ action_type: string; content: string; planned_follow_up_at: string | null }>({
  action_type: 'PROGRESS',
  content: '',
  planned_follow_up_at: null,
})
function openAction() {
  actionForm.action_type = 'PROGRESS'
  actionForm.content = ''
  actionForm.planned_follow_up_at = null
  actionVisible.value = true
}
async function doAction() {
  const d = detail.value!
  if (!actionForm.content) {
    ElMessage.warning('请填写内容')
    return
  }
  busy.value = true
  try {
    await addIssueAction(d.issue.id, d.issue.revision, {
      action_type: actionForm.action_type,
      content: actionForm.content,
      planned_follow_up_at: actionForm.planned_follow_up_at ?? undefined,
    })
    ElMessage.success('已提交')
    actionVisible.value = false
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '提交失败')
  } finally {
    busy.value = false
  }
}

// ---- 申请核验 ----
const verifyRequestVisible = ref(false)
const verifyRequestContent = ref('')
function openRequestVerify() {
  verifyRequestContent.value = ''
  verifyRequestVisible.value = true
}
async function doRequestVerify() {
  const d = detail.value!
  if (!verifyRequestContent.value) {
    ElMessage.warning('请填写核验说明')
    return
  }
  busy.value = true
  try {
    await requestVerification(d.issue.id, d.issue.revision, verifyRequestContent.value)
    ElMessage.success('已申请核验')
    verifyRequestVisible.value = false
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '申请失败')
  } finally {
    busy.value = false
  }
}

// ---- 核验 ----
const verifyVisible = ref(false)
const verifyForm = reactive<{ decision: 'PASS' | 'RETURN'; reason: string }>({ decision: 'PASS', reason: '' })
function openVerify() {
  verifyForm.decision = 'PASS'
  verifyForm.reason = ''
  verifyVisible.value = true
}
async function doVerify() {
  const d = detail.value!
  if (verifyForm.decision === 'RETURN' && !verifyForm.reason) {
    ElMessage.warning('退回必须填写原因')
    return
  }
  busy.value = true
  try {
    await verifyIssue(d.issue.id, d.issue.revision, verifyForm.decision, verifyForm.reason)
    ElMessage.success(verifyForm.decision === 'PASS' ? '已关闭' : '已退回继续处理')
    verifyVisible.value = false
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '核验失败')
  } finally {
    busy.value = false
  }
}

// ---- 重开 ----
async function openReopen() {
  const d = detail.value!
  try {
    const { ElMessageBox } = await import('element-plus')
    const { value } = await ElMessageBox.prompt('填写重新打开原因', '重新打开问题', {
      type: 'warning', inputType: 'textarea',
      confirmButtonText: '重新打开', cancelButtonText: '取消',
      inputValidator: (v: string) => (v?.trim() ? true : '原因必填'),
    })
    await reopenIssue(d.issue.id, d.issue.revision, value.trim())
    ElMessage.success('已重新打开')
    refresh()
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.message || '操作失败')
  }
}

async function refresh() {
  try {
    detail.value = await fetchIssue(detail.value!.issue.id)
  } catch {
    detailVisible.value = false
  }
  load()
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
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
.subtitle {
  margin: 20px 0 10px;
  color: #2b2b2b;
}
.timeline {
  padding-left: 4px;
}
.action-line {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.action-actor {
  font-size: 12px;
  color: #999;
}
.action-content {
  font-size: 13px;
  color: #444;
  white-space: pre-wrap;
}
.ops {
  margin-top: 20px;
  display: flex;
  gap: 10px;
}
</style>
