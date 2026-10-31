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
    <el-dialog v-model="reviewVisible" title="审核记录" width="min(960px, 95vw)" destroy-on-close :close-on-click-modal="false">
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

    <IssueAdmission v-model="admitVisible" :observation-id="admitRecordId" />
  </div>
</template>

<script setup lang="ts">
import { outcomeLabel as outcomeText, observationStatusLabel } from '@/shared/ui/labels'
import { onMounted, ref } from 'vue'
import ObservationEvidence from '@/shared/ui/ObservationEvidence.vue'
import { formatDateTime } from '@/shared/ui/time'
import { ElMessage } from 'element-plus'
import { fetchPendingReviews, reviewObservation, type ReviewDecision } from '@/shared/api/observation'
import IssueAdmission from '@/shared/ui/IssueAdmission.vue'

const reviews = ref<any[]>([])
const loading = ref(false)
const reviewVisible = ref(false)
const current = ref<any>(null)
const decision = ref<ReviewDecision>('ACCEPT')
const reason = ref('')
const submitting = ref(false)

const admitVisible = ref(false)
const admitRecordId = ref(0)

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
    ElMessage.success(`记录状态：${observationStatusLabel(updated.status)}`)
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
