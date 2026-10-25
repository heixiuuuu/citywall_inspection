<template>
  <div class="page">
    <div class="toolbar">
      <el-button type="primary" @click="load">刷新</el-button>
      <span class="hint">待审核记录 · 驳回仅限主动上报</span>
    </div>

    <el-table :data="reviews" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column label="来源" width="100">
        <template #default="{ row }">
          <el-tag :type="row.source === 'TASK' ? 'primary' : 'warning'" size="small">
            {{ row.source === 'TASK' ? '任务' : '主动上报' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="location_text" label="位置" min-width="180" />
      <el-table-column label="现场判断" width="110">
        <template #default="{ row }">{{ outcomeText(row.inspection_outcome) }}</template>
      </el-table-column>
      <el-table-column prop="reporter_id" label="上报人ID" width="100" />
      <el-table-column label="提交时间" width="150">
        <template #default="{ row }">{{ formatDateTime(row.submitted_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openReview(row)">审核</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 审核对话框 -->
    <el-dialog v-model="reviewVisible" title="审核记录" width="min(960px, 95vw)" destroy-on-close>
      <div v-if="current" class="review-body">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="ID">{{ current.id }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ current.source }}</el-descriptions-item>
          <el-descriptions-item label="位置">{{ current.location_text }}</el-descriptions-item>
          <el-descriptions-item label="现场判断">{{ outcomeText(current.inspection_outcome) }}</el-descriptions-item>
          <el-descriptions-item label="上报人ID">{{ current.reporter_id }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ current.revision }}</el-descriptions-item>
        </el-descriptions>
        <ObservationEvidence v-if="reviewVisible" :observation-id="current.id" />
      </div>
      <template #footer>
        <el-select v-model="decision" style="width: 120px; margin-right: 8px">
          <el-option label="通过" value="ACCEPT" />
          <el-option label="退回" value="RETURN" />
          <el-option v-if="current?.source === 'VOLUNTARY'" label="驳回" value="REJECT" />
        </el-select>
        <el-input
          v-model="reason"
          :placeholder="decision === 'RETURN' ? '退回理由（必填）' : decision === 'REJECT' ? '驳回理由（必填）' : '受理说明（可选）'"
          style="width: 300px"
        />
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitReview">提交审核</el-button>
      </template>
    </el-dialog>

    <!-- 受理：审核通过后对记录新建/关联问题或记录不立项原因 -->
    <el-dialog v-model="admitVisible" title="受理问题" width="560">
      <el-form label-width="100px">
        <el-form-item label="受理方式" required>
          <el-radio-group v-model="admitMode">
            <el-radio value="NEW">新建问题</el-radio>
            <el-radio value="LINK">关联已有问题</el-radio>
            <el-radio value="NONE">不立项</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="admitMode === 'NEW'">
          <el-form-item label="标题" required>
            <el-input v-model="admitForm.title" placeholder="如：南门段墙体裂缝" />
          </el-form-item>
          <el-form-item label="优先级" required>
            <el-select v-model="admitForm.priority_code" style="width: 160px">
              <el-option label="关注" value="WATCH" />
              <el-option label="需处理" value="PROCESS" />
              <el-option label="尽快核实" value="URGENT" />
            </el-select>
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="admitForm.description" type="textarea" :rows="2" placeholder="问题描述与初步判断" />
          </el-form-item>
          <el-form-item label="优先级依据">
            <el-input v-model="admitForm.priority_reason" placeholder="如：裂缝延伸至女儿墙，建议尽快核实" />
          </el-form-item>
        </template>
        <template v-else-if="admitMode === 'LINK'">
          <el-form-item label="关联问题" required>
            <el-select v-model="admitForm.link_issue_ids" multiple filterable style="width: 100%" placeholder="选择开放中的问题">
              <el-option v-for="i in openIssues" :key="i.id" :label="`#${i.id} ${i.title}`" :value="i.id" />
            </el-select>
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="不立项原因" required>
            <el-input v-model="admitForm.no_issue_reason" type="textarea" :rows="2" placeholder="如：经核实现场无明显病害，判定为误检" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="admitVisible = false">取消</el-button>
        <el-button type="primary" :loading="admitting" @click="submitAdmit">提交受理</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import ObservationEvidence from '@/shared/ui/ObservationEvidence.vue'
import { formatDateTime } from '@/shared/ui/time'
import { ElMessage } from 'element-plus'
import { fetchPendingReviews, reviewObservation, type ReviewDecision } from '@/shared/api/observation'
import { fetchIssues, admitObservation, type Issue } from '@/shared/api/issue'

const reviews = ref<any[]>([])
const loading = ref(false)
const reviewVisible = ref(false)
const current = ref<any>(null)
const decision = ref<ReviewDecision>('ACCEPT')
const reason = ref('')
const submitting = ref(false)

// ---- 受理（审核通过后） ----
const admitVisible = ref(false)
const admitting = ref(false)
const admitMode = ref<'NEW' | 'LINK' | 'NONE'>('NEW')
const admitRecordId = ref(0)
const openIssues = ref<Issue[]>([])
const admitForm = ref<{
  title: string
  priority_code: 'WATCH' | 'PROCESS' | 'URGENT'
  description: string
  priority_reason: string
  link_issue_ids: number[]
  no_issue_reason: string
}>({ title: '', priority_code: 'WATCH', description: '', priority_reason: '', link_issue_ids: [], no_issue_reason: '' })

onMounted(load)

async function load() {
  loading.value = true
  try {
    reviews.value = await fetchPendingReviews()
  } catch {
    ElMessage.error('加载待审核列表失败')
  } finally {
    loading.value = false
  }
}

function outcomeText(o: string | null) {
  return (
    {
      NORMAL: '正常',
      SUSPECTED: '疑似病害',
      UNDETERMINED: '不确定',
      UNREACHABLE: '不可达',
    }[o ?? ''] ?? '—'
  )
}

async function openReview(row: any) {
  current.value = row
  decision.value = 'ACCEPT'
  reason.value = ''
  reviewVisible.value = true

}

async function submitReview() {
  if (!current.value) return
  if (decision.value !== 'ACCEPT' && !reason.value.trim()) {
    ElMessage.warning('退回/驳回必须填写理由')
    return
  }
  submitting.value = true
  try {
    const payload =
      decision.value === 'ACCEPT'
        ? { revision: current.value.revision, decision: decision.value, disposition: reason.value }
        : { revision: current.value.revision, decision: decision.value, reason: reason.value }
    const updated = await reviewObservation(current.value.id, payload)
    ElMessage.success(`已${decision.value === 'ACCEPT' ? '通过' : decision.value === 'RETURN' ? '退回' : '驳回'}（状态 ${updated.status}）`)
    reviewVisible.value = false
    if (decision.value === 'ACCEPT') {
      openAdmit(current.value.id)
    }
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '审核失败')
  } finally {
    submitting.value = false
  }
}

async function openAdmit(obsId: number) {
  admitRecordId.value = obsId
  admitMode.value = 'NEW'
  admitForm.value = { title: '', priority_code: 'WATCH', description: '', priority_reason: '', link_issue_ids: [], no_issue_reason: '' }
  // 拉取开放问题供关联选择（待分配/已分配/处理中/观察中）。
  try {
    const res = await fetchIssues({ page_size: 100 })
    openIssues.value = res.items.filter((i) => !['CLOSED', 'PENDING_VERIFICATION'].includes(i.status))
  } catch {
    openIssues.value = []
  }
  admitVisible.value = true
}

async function submitAdmit() {
  if (admitMode.value === 'NEW' && !admitForm.value.title.trim()) {
    ElMessage.warning('请填写问题标题')
    return
  }
  if (admitMode.value === 'LINK' && !admitForm.value.link_issue_ids.length) {
    ElMessage.warning('请选择要关联的问题')
    return
  }
  if (admitMode.value === 'NONE' && !admitForm.value.no_issue_reason.trim()) {
    ElMessage.warning('请填写不立项原因')
    return
  }
  admitting.value = true
  try {
    const payload =
      admitMode.value === 'NEW'
        ? { new_issues: [{ title: admitForm.value.title, priority_code: admitForm.value.priority_code, description: admitForm.value.description || undefined, priority_reason: admitForm.value.priority_reason || undefined }] }
        : admitMode.value === 'LINK'
          ? { link_issue_ids: admitForm.value.link_issue_ids }
          : { no_issue_reason: admitForm.value.no_issue_reason }
    const res = await admitObservation(admitRecordId.value, payload)
    const a = res.admit
    if (a.already_admitted) ElMessage.info('该记录已受理（返回已有关联）')
    else ElMessage.success(`受理完成：新建 ${a.created_issue_ids.length} 个，关联 ${a.linked_issue_ids.length} 个`)
    admitVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '受理失败')
  } finally {
    admitting.value = false
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
  gap: 12px;
  margin-bottom: 14px;
}
.hint {
  font-size: 12px;
  color: #9b9b9b;
}
.review-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.subtitle {
  margin: 0 0 8px;
  color: #2b2b2b;
}
.img-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}
.img-cell {
  height: 120px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #e6e1d5;
}
.img-cell :deep(.el-image) {
  width: 100%;
  height: 100%;
}
</style>
