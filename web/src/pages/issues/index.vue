<template>
  <div class="page">
    <PageHeading title="问题台账" description="从现场发现到处置核验，跟进每一项问题。"><span class="result-count">共 {{ total }} 项</span></PageHeading>
    <el-alert v-if="loadError && issues.length" :title="loadError" description="当前保留上次加载的台账内容。" type="error" :closable="false" class="load-notice" />
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索问题名称或位置" clearable style="width: 260px" @clear="search" />
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="resetFilters">重置</el-button>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </div>
    <div class="toolbar">
      <el-checkbox v-model="onlyMine">我负责的</el-checkbox>
      <el-checkbox v-model="activeOnly">仅未关闭</el-checkbox>
      <el-checkbox v-model="followUpDue">到期需复查</el-checkbox>
      <el-radio-group v-model="priority" size="small" aria-label="问题优先级"><el-radio-button value="">全部优先级</el-radio-button><el-radio-button v-for="(item, code) in ISSUE_PRIORITY" :key="code" :value="code">{{ item.label }}</el-radio-button></el-radio-group>
    </div>
    <StatusFilter v-model="activeStatus" :options="statusFilter" label="问题状态筛选" />

    <el-table :data="issues" v-loading="loading" stripe>
      <template #empty><ListFeedback :error="loadError" title="当前没有符合条件的问题" description="可清除名称或状态筛选；新受理的问题会显示在这里。" icon="issues" @retry="load" /></template>
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
      <el-table-column label="责任人" min-width="110"><template #default="{ row }">{{ row.assignee_name || (row.assignee_id ? `用户 #${row.assignee_id}` : '待分配') }}</template></el-table-column>
      <el-table-column label="计划复查" min-width="170"><template #default="{ row }"><span :class="{ 'follow-up-due': isFollowUpDue(row) }">{{ row.next_follow_up_at ? formatDateTime(row.next_follow_up_at) : row.status === 'MONITORING' ? '未安排复查时间' : '—' }}</span><el-tag v-if="isFollowUpDue(row)" size="small" type="warning">已到期</el-tag></template></el-table-column>
      <el-table-column label="开启时间" width="170">
        <template #default="{ row }">{{ formatDateTime(row.opened_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row.id)">跟进</el-button>
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
    <el-drawer v-model="detailVisible" title="问题详情" size="min(860px, 95vw)" :before-close="beforeDetailClose" @closed="clearDetailRoute">
      <el-skeleton v-if="detailLoading" :rows="8" animated />
      <el-result v-else-if="detailFailed" icon="warning" title="问题详情暂时无法加载" sub-title="请检查网络与当前访问权限"><template #extra><el-button @click="loadDetail(Number(route.query.issue_id))">重新加载</el-button></template></el-result>
      <div v-else-if="detail">
        <el-alert v-if="isFollowUpDue(detail.issue)" title="已到计划复查时间，请补充观察记录或处理进展。" type="warning" :closable="false" show-icon class="follow-up-notice" />
        <div class="ops" aria-label="问题处置操作">
          <el-button v-if="isAdmin && canAssign" type="primary" :disabled="busy || photoBusy" @click="openAssign">指派/改派</el-button>
          <el-button v-if="canAct && canAction" type="success" :disabled="busy || photoBusy" @click="openAction">提交进展/观察</el-button>
          <el-button v-if="canAct && canRequestVerify" type="warning" :disabled="busy || photoBusy" @click="openRequestVerify">申请核验</el-button>
          <el-button v-if="canVerify" type="primary" :disabled="busy || photoBusy" @click="openVerify">核验结果</el-button>
          <el-button v-if="canReview && isClosed" :disabled="busy || photoBusy" @click="openReopen">重新打开</el-button>
          <span v-if="awaitingOtherReviewer" class="action-actor">需由另一名审核员核验</span>
        </div>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="标题">{{ detail.issue.title }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="ISSUE_STATUS[detail.issue.status]?.type" size="small">{{ ISSUE_STATUS[detail.issue.status]?.label ?? detail.issue.status }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="优先级">{{ ISSUE_PRIORITY[detail.issue.priority_code]?.label ?? detail.issue.priority_code }}</el-descriptions-item>
          <el-descriptions-item label="责任人">{{ detail.issue.assignee_name || (detail.issue.assignee_id ? `用户 #${detail.issue.assignee_id}` : '待分配') }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.segment_id" label="城墙区段">{{ detail.issue.segment_name || `区段 #${detail.issue.segment_id}` }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.point_id" label="巡检点位">{{ detail.issue.point_name || `点位 #${detail.issue.point_id}` }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.status === 'MONITORING'" label="计划复查" :span="2">{{ detail.issue.next_follow_up_at ? formatDateTime(detail.issue.next_follow_up_at) : '未安排复查时间' }}</el-descriptions-item>
          <el-descriptions-item label="位置" :span="2">{{ detail.issue.location_text }}</el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">{{ detail.issue.description || '—' }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.priority_reason" label="优先级依据" :span="2">{{ detail.issue.priority_reason }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.closed_at" label="关闭时间">{{ formatDateTime(detail.issue.closed_at) }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.issue.close_reason" label="关闭原因" :span="2">{{ detail.issue.close_reason }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ detail.issue.revision }}</el-descriptions-item>
        </el-descriptions>

        <h4 class="subtitle">关联现场记录</h4>
        <p v-if="canAct && detail.observations.length" class="action-actor">展开记录可查看现场照片、识别候选及人工复核。</p>
        <el-table :data="detail.observations" row-key="observation_id" size="small" stripe>
          <el-table-column v-if="canAct" type="expand">
            <template #default="{ row }">
              <ObservationEvidence :observation-id="row.observation_id" />
            </template>
          </el-table-column>
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
              <span class="action-actor">{{ a.actor_name || `用户 #${a.actor_id}` }}</span>
            </div>
            <div class="action-content">{{ a.content }}</div>
            <div v-if="a.planned_follow_up_at" class="action-follow-up">计划复查：{{ formatDateTime(a.planned_follow_up_at) }}</div>
            <IssueActionPhotos v-if="a.images?.length || canAct && canAction && a.actor_id === myId" :issue-id="detail.issue.id" :action-id="a.id" :images="a.images" :can-upload="canAct && canAction && a.actor_id === myId && !busy" @uploading="(pending: boolean) => trackUpload(a.id, pending)" @uploaded="refresh" />
          </el-timeline-item>
        </el-timeline>

      </div>
    </el-drawer>

    <!-- 指派 -->
    <el-dialog v-model="assignVisible" title="指派/改派责任人" width="420" :close-on-click-modal="false" :close-on-press-escape="!busy" :show-close="!busy">
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
        <el-button :disabled="busy" @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doAssign">确认</el-button>
      </template>
    </el-dialog>

    <!-- 处置动作 -->
    <el-dialog v-model="actionVisible" title="提交处置动作" width="480" :close-on-click-modal="false" :close-on-press-escape="!busy" :show-close="!busy">
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
          <p class="schedule-hint">留空表示本次不安排后续复查。</p>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :disabled="busy" @click="actionVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doAction">提交</el-button>
      </template>
    </el-dialog>

    <!-- 申请核验 -->
    <el-dialog v-model="verifyRequestVisible" title="申请核验" width="480" :close-on-click-modal="false" :close-on-press-escape="!busy" :show-close="!busy">
      <el-form label-width="100px">
        <el-form-item label="核验说明" required>
          <el-input v-model="verifyRequestContent" type="textarea" :rows="3" placeholder="说明处理结论与证据情况" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :disabled="busy" @click="verifyRequestVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doRequestVerify">提交</el-button>
      </template>
    </el-dialog>

    <!-- 核验 -->
    <el-dialog v-model="verifyVisible" title="核验问题" width="480" :close-on-click-modal="false" :close-on-press-escape="!busy" :show-close="!busy">
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
        <el-button :disabled="busy" @click="verifyVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="doVerify">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { formatDateTime } from '@/shared/ui/time'
import PageHeading from '@/shared/ui/PageHeading.vue'
import ListFeedback from '@/shared/ui/ListFeedback.vue'
import StatusFilter from '@/shared/ui/StatusFilter.vue'
import ObservationEvidence from '@/shared/ui/ObservationEvidence.vue'
import IssueActionPhotos from '@/shared/ui/IssueActionPhotos.vue'
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
const loadError = ref('')
const route = useRoute()
const router = useRouter()
const keyword = ref(typeof route.query.keyword === 'string' ? route.query.keyword : '')
const priority = ref(typeof route.query.priority_code === 'string' && route.query.priority_code in ISSUE_PRIORITY ? route.query.priority_code : '')
const onlyMine = ref(route.query.only_mine === '1')
const activeOnly = ref(route.query.active_only === '1')
const followUpDue = ref(route.query.follow_up_due === '1')
const activeStatus = ref(typeof route.query.status === 'string' && route.query.status in ISSUE_STATUS ? route.query.status : '')
const page = ref(1)
const pageSize = 20
const total = ref(0)

watch([activeStatus, priority, onlyMine, activeOnly, followUpDue], search)
function search() { page.value = 1; syncFilterQuery(); void load() }
function resetFilters() { keyword.value = ''; activeStatus.value = ''; priority.value = ''; onlyMine.value = false; activeOnly.value = false; followUpDue.value = false; search() }
function isFollowUpDue(item: Issue) { return item.status === 'MONITORING' && !!item.next_follow_up_at && Date.parse(item.next_follow_up_at) <= Date.now() }

const detailVisible = ref(false)
const detail = ref<{
  issue: Issue
  observations: IssueObservationLink[]
  actions: IssueAction[]
} | null>(null)

const busy = ref(false)
const uploadingActions = ref(new Set<number>())
const photoBusy = computed(() => uploadingActions.value.size > 0)
function trackUpload(id: number, pending: boolean) { const next = new Set(uploadingActions.value); if (pending) next.add(id); else next.delete(id); uploadingActions.value = next }
function beforeDetailClose(done: () => void) { if (busy.value || photoBusy.value) { ElMessage.info('正在保存处置内容，请稍候'); return }; done() }
const detailLoading = ref(false)
const detailFailed = ref(false)
let detailGeneration = 0
let listGeneration = 0
watch(() => route.query.issue_id, value => {
  const id = Number(value)
  if (Number.isSafeInteger(id) && id > 0) void loadDetail(id)
  else { detailGeneration++; detailVisible.value = false }
}, { immediate: true })

const canAssign = computed(() => detail.value && ['UNASSIGNED', 'OPEN', 'IN_PROGRESS', 'MONITORING'].includes(detail.value.issue.status))
const canAction = computed(() => !!detail.value && ['OPEN', 'IN_PROGRESS', 'MONITORING'].includes(detail.value.issue.status))
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
  const current = ++listGeneration
  loading.value = true
  loadError.value = ''
  try {
    const res = await fetchIssues({ status: activeStatus.value || undefined, keyword: keyword.value.trim(), priority_code: priority.value, only_mine: onlyMine.value, active_only: activeOnly.value, follow_up_due: followUpDue.value, page: page.value, page_size: pageSize })
    if (current !== listGeneration) return
    issues.value = res.items
    total.value = res.total
  } catch (error) {
    if (current === listGeneration) loadError.value = error instanceof Error ? error.message : '问题台账加载失败，请重新加载。'
  } finally {
    if (current === listGeneration) loading.value = false
  }
}

function openDetail(id: number) { void router.push({ query: { ...route.query, issue_id: String(id) } }) }
function clearDetailRoute() {
  if (!detailVisible.value && route.query.issue_id) { const query = { ...route.query }; delete query.issue_id; void router.replace({ query }) }
}
async function loadDetail(id: number) {
  const current = ++detailGeneration
  detailLoading.value = true
  detailFailed.value = false
  detailVisible.value = true
  detail.value = null
  try {
    const d = await fetchIssue(id)
    if (current !== detailGeneration) return
    detail.value = d
  } catch {
    if (current === detailGeneration) detailFailed.value = true
  } finally { if (current === detailGeneration) detailLoading.value = false }
}

// ---- 指派 ----
const assignVisible = ref(false)
const assignForm = reactive<{ assignee_id: number | undefined; reason: string }>({ assignee_id: undefined, reason: '' })
const assignees = ref<IssueAssignee[]>([])
const assigneeLoading = ref(false)
async function openAssign() {
  if (photoBusy.value || busy.value) return
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
  if (busy.value || photoBusy.value || !detail.value) return
  if (!assignForm.assignee_id || assigneeLoading.value) { ElMessage.warning('请选择责任人'); return }
  const d = detail.value!
  busy.value = true
  try {
    await assignIssue(d.issue.id, d.issue.revision, assignForm.assignee_id, assignForm.reason)
    ElMessage.success('已指派')
    assignVisible.value = false
    await refresh()
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
  if (photoBusy.value || busy.value) return
  actionForm.action_type = 'PROGRESS'
  actionForm.content = ''
  actionForm.planned_follow_up_at = null
  actionVisible.value = true
}
async function doAction() {
  if (busy.value || photoBusy.value || !detail.value) return
  const d = detail.value!
  if (!actionForm.content.trim()) {
    ElMessage.warning('请填写内容')
    return
  }
  busy.value = true
  try {
    await addIssueAction(d.issue.id, d.issue.revision, {
      action_type: actionForm.action_type,
      content: actionForm.content.trim(),
      planned_follow_up_at: actionForm.action_type === 'OBSERVE' ? actionForm.planned_follow_up_at ?? undefined : undefined,
    })
    ElMessage.success('已提交')
    actionVisible.value = false
    await refresh()
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
  if (photoBusy.value || busy.value) return
  verifyRequestContent.value = ''
  verifyRequestVisible.value = true
}
async function doRequestVerify() {
  if (busy.value || photoBusy.value || !detail.value) return
  const d = detail.value!
  if (!verifyRequestContent.value.trim()) {
    ElMessage.warning('请填写核验说明')
    return
  }
  busy.value = true
  try {
    await requestVerification(d.issue.id, d.issue.revision, verifyRequestContent.value.trim())
    ElMessage.success('已申请核验')
    verifyRequestVisible.value = false
    await refresh()
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
  if (photoBusy.value || busy.value) return
  verifyForm.decision = 'PASS'
  verifyForm.reason = ''
  verifyVisible.value = true
}
async function doVerify() {
  if (busy.value || photoBusy.value || !detail.value) return
  const d = detail.value!
  if (verifyForm.decision === 'RETURN' && !verifyForm.reason.trim()) {
    ElMessage.warning('退回必须填写原因')
    return
  }
  busy.value = true
  try {
    await verifyIssue(d.issue.id, d.issue.revision, verifyForm.decision, verifyForm.reason.trim())
    ElMessage.success(verifyForm.decision === 'PASS' ? '已关闭' : '已退回继续处理')
    verifyVisible.value = false
    await refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '核验失败')
  } finally {
    busy.value = false
  }
}

// ---- 重开 ----
async function openReopen() {
  if (busy.value || photoBusy.value || !detail.value) return
  const d = detail.value!
  try {
    const { ElMessageBox } = await import('element-plus')
    const { value } = await ElMessageBox.prompt('填写重新打开原因', '重新打开问题', {
      type: 'warning', inputType: 'textarea',
      confirmButtonText: '重新打开', cancelButtonText: '取消',
      inputValidator: (v: string) => (v?.trim() ? true : '原因必填'),
    })
    busy.value = true
    await reopenIssue(d.issue.id, d.issue.revision, value.trim())
    ElMessage.success('已重新打开')
    await refresh()
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.message || '操作失败')
  } finally { busy.value = false }
}

async function refresh() {
  const id = detail.value?.issue.id
  if (!id) return
  const wasBusy = busy.value
  busy.value = true
  const current = ++detailGeneration
  const results = await Promise.allSettled([fetchIssue(id), load()])
  busy.value = wasBusy
  if (current !== detailGeneration || !detailVisible.value) return
  const result = results[0]
  if (result.status === 'fulfilled') detail.value = result.value
  else { detail.value = null; detailFailed.value = true; ElMessage.warning('操作已保存，详情刷新失败，请重新加载') }
}

function syncFilterQuery() {
  const query = { ...route.query }
  for (const key of ['keyword', 'priority_code', 'status', 'only_mine', 'active_only', 'follow_up_due']) delete query[key]
  if (keyword.value.trim()) query.keyword = keyword.value.trim()
  if (priority.value) query.priority_code = priority.value
  if (activeStatus.value) query.status = activeStatus.value
  if (onlyMine.value) query.only_mine = '1'
  if (activeOnly.value) query.active_only = '1'
  if (followUpDue.value) query.follow_up_due = '1'
  void router.replace({ query })
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
.action-follow-up {
  margin-top: 4px;
  font-size: 13px;
  color: #444;
}
.ops {
  margin-bottom: 20px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.result-count { font-size: 13px; white-space: nowrap; color: var(--text-muted); }
.follow-up-due { display: block; color: var(--el-color-warning-dark-2); }
.follow-up-notice { margin-bottom: var(--space-content); }
.schedule-hint { margin: 4px 0 0; font-size: 12px; color: var(--text-muted); }
</style>
