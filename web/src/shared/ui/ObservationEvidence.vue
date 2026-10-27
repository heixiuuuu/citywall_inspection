<template>
  <section v-loading="loading" class="evidence">
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <template v-if="evidence">
      <p>经纬度：{{ evidence.observation.latitude ?? '—' }}, {{ evidence.observation.longitude ?? '—' }} · 精度 {{ evidence.observation.accuracy_m ?? '—' }} m</p>
      <p v-if="evidence.observation.admitted_at">受理：{{ evidence.observation.no_issue_reason ? `不立项 · ${evidence.observation.no_issue_reason}` : '已立项或关联问题' }} · {{ formatDateTime(evidence.observation.admitted_at) }}</p>
      <p v-else-if="evidence.observation.status === 'ACCEPTED'" class="hint">受理：尚未受理</p>
      <h4>现场照片与检测框</h4>
      <p class="hint">候选仅供复核。未检出不能代替人工判断，置信度不表示严重程度。</p>
      <article v-for="img in images" :key="img.id">
        <div class="image-toolbar">
          <span>{{ img.role === 'OVERVIEW' ? '概览' : '细节' }} #{{ img.id }}</span>
          <el-select v-if="attemptsFor(img.id).length" v-model="selected[img.id]" style="width: 260px" size="small">
            <el-option v-for="a in attemptsFor(img.id)" :key="a.id" :value="a.id" :label="`第 ${a.attempt_no} 次 · ${a.model_version || a.status}`" />
          </el-select>
          <span v-else class="hint">尚未识别</span>
        </div>
        <div v-if="img.url" class="image-stage">
          <img :src="img.url" :alt="`现场照片 #${img.id}`" @load="measure(img.id, $event)" />
          <svg v-if="sizes[img.id]" :viewBox="`0 0 ${sizes[img.id][0]} ${sizes[img.id][1]}`" preserveAspectRatio="none">
            <g v-for="d in detectionsFor(img.id)" :key="d.id">
              <rect :x="d.x1" :y="d.y1" :width="d.x2-d.x1" :height="d.y2-d.y1" fill="none" stroke="#ffd54f" stroke-width="2" vector-effect="non-scaling-stroke" />
              <text :x="d.x1" :y="Math.max(18, d.y1-5)" fill="#ffd54f" stroke="#222" stroke-width="0.6" paint-order="stroke" :font-size="Math.max(14, sizes[img.id][0]/45)">{{ d.class_code }} {{ (d.confidence*100).toFixed(1) }}%</text>
            </g>
          </svg>
        </div>
        <el-alert v-else title="照片加载失败" type="warning" :closable="false" />
        <p v-if="attemptFor(img.id)" class="hint">{{ attemptFor(img.id)?.status }} {{ attemptFor(img.id)?.error_code }} · 候选 {{ detectionsFor(img.id).length }} 个</p>
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
      <h4>人工结论与修订</h4>
      <article v-for="c in evidence.conclusions" :key="c.id" class="conclusion">
        <strong>第 {{ c.revision_no }} 版 · {{ c.outcome }}</strong>
        <p>{{ c.description || '未填写描述' }}</p>
        <el-table :data="evidence.items.filter(i => i.conclusion_revision_id === c.id)" size="small">
          <el-table-column prop="image_id" label="图片" width="65" />
          <el-table-column prop="decision" label="判定" width="90" />
          <el-table-column prop="final_class_code" label="类别" min-width="120" />
          <el-table-column prop="description" label="说明" min-width="150" />
        </el-table>
      </article>
      <el-empty v-if="!evidence.conclusions.length" description="暂无人工结论" :image-size="50" />
      <h4 v-if="evidence.reviews.length">审核记录</h4>
      <p v-for="r in evidence.reviews" :key="r.id">{{ r.decision }} · {{ r.reason || r.disposition || '无附加说明' }}</p>
    </template>
  </section>
</template>
<script setup lang="ts">
import { formatDateTime } from '@/shared/ui/time'
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { fetchEvidence, fetchObservationImages, loadImageBlob, type Evidence, type ObservationImageMeta, type VisionAnalysis } from '@/shared/api/observation'
const props = defineProps<{ observationId: number }>()
const evidence = ref<Evidence>()
const images = ref<(ObservationImageMeta & { url: string })[]>([])
const selected = reactive<Record<number, number>>({})
const sizes = reactive<Record<number, [number, number]>>({})
const loading = ref(false)
const error = ref('')
let generation = 0
function release() { images.value.forEach(i => { if (i.url) URL.revokeObjectURL(i.url) }); images.value = [] }
watch(() => props.observationId, async id => {
  const current = ++generation
  release()
  evidence.value = undefined
  loading.value = true
  error.value = ''
  try {
    const [detail, metas] = await Promise.all([fetchEvidence(id), fetchObservationImages(id)])
    if (current !== generation) return
    evidence.value = detail
    for (const a of detail.attempts) selected[a.image_id] = a.id
    const loaded = await Promise.all(metas.map(async m => ({ ...m, url: await loadImageBlob(m.id).catch(() => '') })))
    if (current !== generation) { loaded.forEach(i => { if (i.url) URL.revokeObjectURL(i.url) }); return }
    images.value = loaded
  } catch (e) { if (current === generation) error.value = e instanceof Error ? e.message : '记录加载失败' }
  finally { if (current === generation) loading.value = false }
}, { immediate: true })
onBeforeUnmount(() => { generation++; release() })
function attemptsFor(id: number) { return evidence.value?.attempts.filter(a => a.image_id === id) ?? [] }
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
.image-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.image-stage { position: relative; line-height: 0; background: #182c29; }
.image-stage img { width: 100%; height: auto; }
.image-stage svg { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
.analysis, .conclusion { background: #f5f7f7; padding: 12px; border-radius: 8px; }
</style>
