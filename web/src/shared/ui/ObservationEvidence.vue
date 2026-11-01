<template>
  <section v-loading="loading" class="evidence">
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-button v-if="error" class="evidence-retry" @click="loadEvidence(props.observationId)">重新加载证据</el-button>
    <template v-if="evidence">
      <div v-if="evidence.submissions?.length" class="image-toolbar">
        <span>查看版本</span>
        <el-select v-model="submissionId" style="width: 340px" aria-label="查看提交版本">
          <el-option :value="0" label="当前记录" />
          <el-option v-for="submission in evidence.submissions" :key="submission.id" :value="submission.id" :label="`第 ${submission.submission_no} 次提交 · ${formatDateTime(submission.submitted_at)}`" />
        </el-select>
      </div>
      <p v-if="snapshot" class="hint">提交快照只读，保留本次提交的位置、照片和人工结论。</p>
      <p v-if="snapshot">提交位置：{{ location?.location_text || '未填写' }}</p>
      <el-alert v-if="evidence.task_status === 'CANCELLED'" title="任务已取消，保留已有证据供查阅。" type="info" :closable="false" />
      <p>经纬度：{{ location?.latitude ?? '—' }}, {{ location?.longitude ?? '—' }} · 精度 {{ location?.accuracy_m ?? '—' }} m</p>
      <p v-if="location?.distance_m != null">距任务目标 {{ location.distance_m.toFixed(1) }} m · 提示半径 {{ locationRadius ?? '—' }} m</p>
      <el-alert v-if="locationWarning" :title="locationWarning" type="warning" :closable="false" />
      <p v-if="evidence.observation.admitted_at">受理：{{ evidence.observation.no_issue_reason ? `不立项 · ${evidence.observation.no_issue_reason}` : '已立项或关联问题' }} · {{ formatDateTime(evidence.observation.admitted_at) }}</p>
      <p v-else-if="evidence.observation.status === 'ACCEPTED'" class="hint">受理：尚未受理</p>
      <h4>现场照片与检测框</h4>
      <p class="hint">候选仅供复核。剥落识别可能误检或漏检；未检出不能代替人工判断，置信度不表示严重程度。</p>
      <article v-for="img in images" :key="img.id">
        <div class="image-toolbar">
          <span>{{ img.role === 'OVERVIEW' ? '概览' : img.role === 'DETAIL' ? '细节' : '历史照片' }} #{{ img.id }}</span>
          <el-button v-if="img.url" link type="primary" @click="openPhoto(img.id)">放大照片</el-button>
          <el-select v-if="attemptsFor(img.id).length" v-model="selected[img.id]" style="width: 260px" size="small">
            <el-option v-for="a in attemptsFor(img.id)" :key="a.id" :value="a.id" :label="`第 ${a.attempt_no} 次 · ${a.model_version || attemptLabel(a.status)}`" />
          </el-select>
          <span v-else class="hint">{{ snapshot ? '提交时无已完成识别' : '尚未识别' }}</span>
        </div>
        <div v-if="img.url" class="image-stage">
          <img :src="img.url" :alt="`现场照片 #${img.id}`" role="button" tabindex="0" aria-label="放大查看现场照片" @click="openPhoto(img.id)" @keydown.enter.prevent="openPhoto(img.id)" @keydown.space.prevent="openPhoto(img.id)" @load="measure(img.id, $event)" />
          <svg v-if="sizes[img.id]" :viewBox="`0 0 ${sizes[img.id][0]} ${sizes[img.id][1]}`" preserveAspectRatio="none">
            <g v-for="d in detectionsFor(img.id)" :key="d.id">
              <rect :x="d.x1" :y="d.y1" :width="d.x2-d.x1" :height="d.y2-d.y1" fill="none" stroke="#ffd54f" stroke-width="2" vector-effect="non-scaling-stroke" />
              <text :x="d.x1" :y="Math.max(18, d.y1-5)" fill="#ffd54f" stroke="#222" stroke-width="0.6" paint-order="stroke" :font-size="Math.max(14, sizes[img.id][0]/45)">{{ categoryLabel(d.class_code) }} {{ (d.confidence*100).toFixed(1) }}%</text>
            </g>
            <g v-for="box in manualBoxes(img.id)" :key="box.id">
              <rect :x="box.x1" :y="box.y1" :width="box.x2-box.x1" :height="box.y2-box.y1" fill="none" stroke="#e15d48" stroke-width="2" vector-effect="non-scaling-stroke" />
              <text :x="box.x1" :y="Math.max(18, box.y1-5)" fill="#e15d48" stroke="#222" stroke-width="0.6" paint-order="stroke" :font-size="Math.max(14, sizes[img.id][0]/45)">人工 · {{ categoryLabel(box.classCode) }}</text>
            </g>
          </svg>
        </div>
        <div v-else><el-alert title="照片加载失败，可重新加载后查看。" type="warning" :closable="false" /><el-button text type="primary" @click="loadImages">重新加载照片</el-button></div>
        <p v-if="attemptFor(img.id)" class="hint">{{ attemptLabel(attemptFor(img.id)?.status) }} {{ attemptFor(img.id)?.error_code }} · 候选 {{ detectionsFor(img.id).length }} 个</p>
        <div v-if="analysisFor(img.id)" class="analysis">
          <h4>视觉语言模型辅助分析</h4>
          <template v-if="analysisFor(img.id)?.status === 'SUCCEEDED'">
            <p>{{ analysisFor(img.id)?.description }}</p>
            <p>可能成因：{{ analysisFor(img.id)?.possible_causes.join('；') || '无法判断' }}</p>
            <p>复查建议：{{ analysisFor(img.id)?.recommendations.join('；') || '需人工复查' }}</p>
            <p class="hint">模型 {{ analysisFor(img.id)?.model }} · 须经人工复核。</p>
          </template>
          <p v-else class="hint">{{ analysisFor(img.id)?.status === 'DISABLED' ? '未配置视觉语言模型' : '辅助分析失败' }} · {{ analysisFor(img.id)?.error_code }}</p>
        </div>
      </article>
      <el-empty v-if="!images.length" description="暂无现场照片" :image-size="50" />
      <h4>{{ snapshot ? '本次提交人工结论' : '人工结论与修订' }}</h4>
      <article v-for="c in conclusions" :key="c.id" class="conclusion">
        <strong>第 {{ c.revision_no }} 版 · {{ outcomeLabel(c.outcome) }}</strong>
        <p>{{ c.description || '未填写描述' }}</p>
        <el-table :data="evidence.items.filter(i => i.conclusion_revision_id === c.id)" size="small">
          <el-table-column prop="image_id" label="图片" width="65" />
          <el-table-column label="判定" width="90"><template #default="{ row }">{{ decisionLabel(row.decision) }}</template></el-table-column>
          <el-table-column label="类别" min-width="120"><template #default="{ row }">{{ categoryLabel(row.final_class_code) }}</template></el-table-column>
          <el-table-column prop="description" label="说明" min-width="150" />
        </el-table>
      </article>
      <el-empty v-if="!conclusions.length" description="暂无人工结论" :image-size="50" />
      <h4 v-if="reviews.length">审核记录</h4>
      <article v-for="r in reviews" :key="r.id">
        <p>{{ reviewLabel(r.decision) }} · {{ formatDateTime(r.created_at) }}</p>
        <p v-if="r.disposition">处置意见：{{ r.disposition }}</p>
        <p v-if="r.reason">审核原因：{{ r.reason }}</p>
      </article>
    </template>
    <el-image-viewer v-if="viewingPhoto" :url-list="previewUrls" :initial-index="previewIndex" :teleported="true" @close="viewingPhoto = false" />
  </section>
</template>
<script setup lang="ts">
import { formatDateTime } from '@/shared/ui/time'
import { categoryLabel, outcomeLabel, reviewLabel, decisionLabel, attemptLabel } from './labels'
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { fetchEvidence, fetchObservationImages, loadImageBlob, type Evidence, type ObservationImageMeta, type VisionAnalysis } from '@/shared/api/observation'
const props = defineProps<{ observationId: number }>()
const evidence = ref<Evidence>()
const submissionId = ref(0)
const currentMetas = ref<ObservationImageMeta[]>([])
const snapshot = computed(() => evidence.value?.submissions?.find(submission => submission.id === submissionId.value))
const location = computed(() => snapshot.value?.location ?? evidence.value?.observation)
const locationRadius = computed(() => snapshot.value?.location.radius_m ?? evidence.value?.location_radius_m)
const locationWarning = computed(() => [
  location.value?.distance_m != null && locationRadius.value != null && location.value.distance_m > locationRadius.value ? '超出任务点提示半径，请结合现场情况复核' : '',
  location.value?.accuracy_m != null && location.value.accuracy_m > 30 ? '定位精度较低，请复核位置' : '',
].filter(Boolean).join('；'))
const conclusions = computed(() => evidence.value?.conclusions.filter(conclusion => !snapshot.value || conclusion.id === snapshot.value.conclusion_revision_id) ?? [])
function manualBoxes(imageId: number) {
  const current = conclusions.value[conclusions.value.length - 1]
  if (!current) return []
  return (evidence.value?.items ?? []).filter(item => item.conclusion_revision_id === current.id && item.image_id === imageId && item.decision === 'ADD' && item.final_box_json).flatMap(item => {
    try {
      const box = JSON.parse(item.final_box_json)
      if (!['x1', 'y1', 'x2', 'y2'].every(key => typeof box[key] === 'number' && Number.isFinite(box[key]))) return []
      return [{ id: item.id, classCode: item.final_class_code, x1: box.x1 as number, y1: box.y1 as number, x2: box.x2 as number, y2: box.y2 as number }]
    } catch { return [] }
  })
}
const reviews = computed(() => evidence.value?.reviews.filter(review => !snapshot.value || review.submission_id === snapshot.value.id) ?? [])
const images = ref<(ObservationImageMeta & { url: string })[]>([])
const selected = reactive<Record<number, number>>({})
const sizes = reactive<Record<number, [number, number]>>({})
const loading = ref(false)
const error = ref('')
const viewingPhoto = ref(false)
const previewIndex = ref(0)
const previewUrls = computed(() => images.value.filter(image => image.url).map(image => image.url))
function openPhoto(id: number) { const image = images.value.find(image => image.id === id); if (!image?.url) return; previewIndex.value = previewUrls.value.indexOf(image.url); viewingPhoto.value = true }
let generation = 0
function release() { viewingPhoto.value = false; images.value.forEach(i => { if (i.url) URL.revokeObjectURL(i.url) }); images.value = [] }
async function loadEvidence(id: number) {
  const current = ++generation
  release()
  evidence.value = undefined
  currentMetas.value = []
  submissionId.value = 0
  loading.value = true
  error.value = ''
  try {
    const [detail, metas] = await Promise.all([fetchEvidence(id), fetchObservationImages(id)])
    if (current !== generation) return
    currentMetas.value = metas
    evidence.value = detail
    await loadImages()
  } catch (e) { if (current === generation) error.value = e instanceof Error ? e.message : '记录加载失败' }
  finally { if (current === generation) loading.value = false }
}
watch(() => props.observationId, loadEvidence, { immediate: true })
watch(submissionId, () => { if (evidence.value) loadImages() })
async function loadImages() {
  const current = ++generation
  release()
  loading.value = true
  const metas = snapshot.value?.images ?? currentMetas.value
  for (const image of metas) {
    const attempts = attemptsFor(image.id)
    if (attempts.length) selected[image.id] = attempts[attempts.length - 1].id
    else delete selected[image.id]
  }
  const loaded = await Promise.all(metas.map(async m => ({ ...m, url: await loadImageBlob(m.id).catch(() => '') })))
  if (current !== generation) { loaded.forEach(i => { if (i.url) URL.revokeObjectURL(i.url) }); return }
  images.value = loaded
  loading.value = false
}
onBeforeUnmount(() => { generation++; release() })
function attemptsFor(id: number) { return evidence.value?.attempts.filter(a => a.image_id === id && (!snapshot.value || a.finished_at && Date.parse(a.finished_at) <= Date.parse(snapshot.value.submitted_at))) ?? [] }
function attemptFor(id: number) { return attemptsFor(id).find(a => a.id === selected[id]) }
function detectionsFor(id: number) { return evidence.value?.detections.filter(d => d.attempt_id === selected[id]) ?? [] }
function analysisFor(id: number): VisionAnalysis | null {
  try { return JSON.parse(attemptFor(id)?.analysis_json || 'null') } catch { return null }
}
function measure(id: number, event: Event) {
  const image = event.target as HTMLImageElement
  sizes[id] = [image.naturalWidth, image.naturalHeight]
}
</script>
<style scoped>
.evidence { min-height: 100px; }
.hint { color: #777; font-size: 12px; }
article { margin-bottom: 20px; }
.image-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-bottom: 8px; }
.image-stage { position: relative; line-height: 0; background: #182c29; }
.image-stage img { width: 100%; height: auto; cursor: zoom-in; }
.image-stage img:focus-visible { outline: none; box-shadow: var(--focus-soft); }
.evidence-retry { margin-top: var(--space-tight); }
.image-stage svg { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
.analysis, .conclusion { background: #f5f7f7; padding: 12px; border-radius: 8px; }
</style>
