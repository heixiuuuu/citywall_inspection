<template>
  <div class="page">
    <PageHeading title="审核管理" description="结合现场照片和人工结论复核记录，确认后继续受理或退回补正。"><span class="queue-count">{{ reviews.length }} 条待审核</span></PageHeading>
    <el-alert v-if="loadError && reviews.length" :title="loadError" description="当前保留上次加载的队列。" type="error" :closable="false" class="load-notice" />
    <div class="toolbar">
      <el-button :loading="loading" @click="load">刷新队列</el-button>
      <span class="hint">本人上报的记录由其他审核员处理。</span>
    </div>

    <el-table :data="reviews" v-loading="loading" stripe>
      <template #empty><ListFeedback :error="loadError" title="待审核记录已处理完毕" description="新的现场记录提交后会出现在这里；已通过记录可在记录检索中继续受理。" icon="reviews" @retry="load"><el-button text type="primary" @click="$router.push('/records?pending_admission=1')">查看待受理记录</el-button></ListFeedback></template>
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
          <el-button link type="primary" @click="openReview(row)">{{ row.reporter_id === myId ? '查看记录' : '开始审核' }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 审核对话框 -->
    <el-dialog v-model="reviewVisible" :title="`审核现场记录 #${current?.id ?? ''}`" width="min(960px, 95vw)" top="4vh" class="review-dialog" destroy-on-close :close-on-click-modal="false" :close-on-press-escape="!submitting" :show-close="!submitting">
      <div v-if="current" class="review-body">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="ID">{{ current.id }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ current.source === 'TASK' ? '任务巡检' : '主动上报' }}</el-descriptions-item>
          <el-descriptions-item label="位置">{{ current.location_text }}</el-descriptions-item>
          <el-descriptions-item label="现场判断">{{ outcomeText(current.inspection_outcome) }}</el-descriptions-item>
          <el-descriptions-item label="上报人ID">{{ current.reporter_id }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ current.revision }}</el-descriptions-item>
        </el-descriptions>
        <ObservationEvidence v-if="reviewVisible" :observation-id="current.id" />
      </div>
      <template #footer>
        <el-alert v-if="current?.reporter_id === myId" title="这是本人上报的记录，请交由其他审核员完成审核。" type="info" :closable="false" />
        <div v-else class="review-footer">
          <el-radio-group v-model="decision" :disabled="submitting" aria-label="审核结论"><el-radio-button value="ACCEPT">通过</el-radio-button><el-radio-button value="RETURN">退回补正</el-radio-button><el-radio-button v-if="current?.source === 'VOLUNTARY'" value="REJECT">驳回上报</el-radio-button></el-radio-group>
          <p class="decision-hint" :class="decision.toLowerCase()">{{ decisionHint }}</p>
          <el-input v-model="reason" :disabled="submitting" type="textarea" :rows="2" :aria-label="decision === 'ACCEPT' ? '审核处置意见' : '审核原因'" :placeholder="decision === 'RETURN' ? '请说明需要补充的照片、位置或结论，方便巡检员准确补正' : decision === 'REJECT' ? '请填写驳回原因，便于上报人了解处理结果' : '审核处置意见（选填）'" />
          <p v-if="submitError" class="submit-error" role="alert">{{ submitError }}</p>
        </div>
        <div class="footer-actions"><el-button :disabled="submitting" @click="reviewVisible = false">{{ current?.reporter_id === myId ? '关闭' : '暂不处理' }}</el-button><el-button v-if="current?.reporter_id !== myId" :type="decision === 'ACCEPT' ? 'primary' : decision === 'RETURN' ? 'warning' : 'danger'" :loading="submitting" @click="submitReview">{{ decision === 'ACCEPT' ? '通过并继续受理' : decision === 'RETURN' ? '确认退回补正' : '确认驳回上报' }}</el-button></div>
      </template>
    </el-dialog>

    <IssueAdmission v-model="admitVisible" :observation-id="admitRecordId" />
  </div>
</template>

<script setup lang="ts">
import { outcomeLabel as outcomeText } from '@/shared/ui/labels'
import { computed, onMounted, ref } from 'vue'
import PageHeading from '@/shared/ui/PageHeading.vue'
import ListFeedback from '@/shared/ui/ListFeedback.vue'
import ObservationEvidence from '@/shared/ui/ObservationEvidence.vue'
import { formatDateTime } from '@/shared/ui/time'
import { ElMessage } from 'element-plus'
import { fetchPendingReviews, reviewObservation, type ReviewDecision, type Observation } from '@/shared/api/observation'
import IssueAdmission from '@/shared/ui/IssueAdmission.vue'

const reviews = ref<Observation[]>([])
const loading = ref(false)
const loadError = ref('')
const submitError = ref('')
const myId = Number(JSON.parse(localStorage.getItem('city_wall_user') ?? 'null')?.id ?? 0)
const reviewVisible = ref(false)
const current = ref<Observation | null>(null)
const decision = ref<ReviewDecision>('ACCEPT')
const reason = ref('')
const submitting = ref(false)
const decisionHint = computed(() => ({ ACCEPT: '通过后将继续受理，可新建问题、关联已有问题或记录不立项原因。', RETURN: '退回后，巡检员可补充现场证据并重新提交。', REJECT: '驳回后，本条主动上报结束处理；如需补充材料，请选择退回补正。' }[decision.value]))

const admitVisible = ref(false)
const admitRecordId = ref(0)

onMounted(load)

async function load() {
  if (loading.value) return
  loading.value = true
  loadError.value = ''
  try {
    reviews.value = await fetchPendingReviews() ?? []
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '审核队列加载失败，请重新加载。'
  } finally {
    loading.value = false
  }
}


function openReview(row: Observation) {
  current.value = row
  decision.value = 'ACCEPT'
  reason.value = ''
  submitError.value = ''
  reviewVisible.value = true

}

async function submitReview() {
  if (!current.value || submitting.value || current.value.reporter_id === myId) return
  submitError.value = ''
  if (decision.value !== 'ACCEPT' && !reason.value.trim()) {
    submitError.value = '请填写具体原因后再提交。'
    return
  }
  submitting.value = true
  try {
    const payload =
      decision.value === 'ACCEPT'
        ? { revision: current.value.revision, decision: decision.value, disposition: reason.value.trim() }
        : { revision: current.value.revision, decision: decision.value, reason: reason.value.trim() }
    const updated = await reviewObservation(current.value.id, payload)
    ElMessage.success(updated.status === 'ACCEPTED' ? '审核已通过，继续填写受理信息。' : updated.status === 'RETURNED' ? '已退回补正，巡检员可查看原因并重新提交。' : '已驳回这条主动上报。')
    reviewVisible.value = false
    if (decision.value === 'ACCEPT') {
      openAdmit(current.value.id)
    }
    load()
  } catch (error) {
    submitError.value = error instanceof Error ? error.message : '审核未能保存，请稍后重试。'
  } finally {
    submitting.value = false
  }
}

function openAdmit(obsId: number) {
  admitRecordId.value = obsId
  admitVisible.value = true
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
  color: var(--text-muted);
}
.review-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.queue-count { font-size: var(--font-small); color: var(--brand); background: var(--surface-hover); border-radius: var(--radius-control); padding: 6px 12px; }
.review-footer { text-align: left; background: var(--surface-subtle); padding: var(--space-content); border-radius: var(--radius-panel); }
.decision-hint { margin: var(--space-tight) 0; font-size: var(--font-small); color: var(--text-muted); }
.decision-hint.return { color: var(--feedback-warning); }
.decision-hint.reject, .submit-error { color: var(--feedback-danger); }
.submit-error { margin: var(--space-tight) 0 0; font-size: var(--font-small); }
.footer-actions { display: flex; gap: var(--space-tight); justify-content: flex-end; margin-top: var(--space-content); }
.review-body { max-height: 45vh; overflow-y: auto; padding-right: var(--space-tight); }
@media (max-width: 600px) { .review-body { max-height: 36vh; } .toolbar { flex-wrap: wrap; } }
</style>
