<template>
  <div class="page" v-loading="loading">
    <div class="toolbar">
      <el-button type="primary" @click="editPoint()">新建点位</el-button>
      <el-button @click="editSegment()">新建城墙区段</el-button>
      <el-button @click="load">刷新</el-button>
    </div>
    <div class="archive-summary"><strong>{{ segments.length }}</strong> 个城墙区段 <span>·</span> <strong>{{ points.length }}</strong> 个巡检点位 <span>·</span> <strong>{{ usablePoints }}</strong> 个可派发点位</div>
    <div class="toolbar filters">
      <el-input v-model="keyword" clearable placeholder="按名称、编码或位置搜索" style="width: 260px" />
      <el-select v-model="segmentFilter" clearable filterable placeholder="全部城墙区段" style="width: 200px"><el-option v-for="segment in segments" :key="segment.id" :value="segment.id" :label="segment.name" /></el-select>
      <el-radio-group v-model="activeFilter"><el-radio-button value="all">全部</el-radio-button><el-radio-button value="active">可用</el-radio-button><el-radio-button value="inactive">停用</el-radio-button></el-radio-group>
    </div>
    <el-tabs>
      <el-tab-pane label="巡检点位">
        <el-table :data="filteredPoints" stripe empty-text="没有匹配的巡检点位">
          <el-table-column prop="code" label="编码" width="150" />
          <el-table-column prop="name" label="名称" min-width="150" />
          <el-table-column label="区段" min-width="140"><template #default="{row}">{{ segments.find(s => s.id === row.segment_id)?.name || row.segment_id }}</template></el-table-column>
          <el-table-column label="经纬度" min-width="200"><template #default="{row}">{{ row.latitude.toFixed(6) }}, {{ row.longitude.toFixed(6) }}</template></el-table-column>
          <el-table-column prop="radius_m" label="定位半径(m)" width="110" />
          <el-table-column prop="location_hint" label="位置提示" min-width="150" />
          <el-table-column label="状态" width="120"><template #default="{row}"><el-tag :type="pointUsable(row) ? 'success' : 'info'">{{ !row.active ? '点位停用' : pointUsable(row) ? '可用' : '区段停用' }}</el-tag></template></el-table-column>
          <el-table-column label="操作" width="80" fixed="right"><template #default="{row}"><el-button link type="primary" @click="editPoint(row)">编辑</el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="城墙档案">
        <el-table :data="filteredSegments" stripe empty-text="没有匹配的城墙区段">
          <el-table-column prop="code" label="编码" width="150" />
          <el-table-column prop="name" label="区段名称" min-width="150" />
          <el-table-column prop="description" label="档案说明" min-width="280" />
          <el-table-column label="点位数" width="100"><template #default="{row}">{{ points.filter(point => point.segment_id === row.id).length }}</template></el-table-column>
          <el-table-column label="状态" width="90"><template #default="{row}">{{ row.active ? '启用' : '停用' }}</template></el-table-column>
          <el-table-column label="操作" width="80"><template #default="{row}"><el-button link type="primary" @click="editSegment(row)">编辑</el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
    <el-dialog v-model="pointVisible" :title="pointId ? '编辑点位' : '新建点位'" width="560" :close-on-click-modal="false">
      <el-form label-width="100px">
        <el-form-item label="区段" required><el-select v-model="pointForm.segment_id" filterable><el-option v-for="s in segments" :key="s.id" :label="`${s.name}${s.active ? '' : '（已停用）'}`" :value="s.id" /></el-select></el-form-item>
        <el-form-item label="编码" required><el-input v-model="pointForm.code" maxlength="32" /></el-form-item>
        <el-form-item label="名称" required><el-input v-model="pointForm.name" maxlength="128" /></el-form-item>
        <el-form-item label="纬度" required><el-input-number v-model="pointForm.latitude" :min="-90" :max="90" :precision="7" placeholder="请输入纬度" /></el-form-item>
        <el-form-item label="经度" required><el-input-number v-model="pointForm.longitude" :min="-180" :max="180" :precision="7" placeholder="请输入经度" /></el-form-item>
        <el-form-item label="定位半径(m)"><el-input-number v-model="pointForm.radius_m" :min="1" :max="500" /></el-form-item>
        <el-form-item label="位置提示"><el-input v-model="pointForm.location_hint" maxlength="255" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="pointForm.active" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="pointVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitPoint">保存</el-button></template>
    </el-dialog>
    <el-dialog v-model="segmentVisible" :title="segmentId ? '编辑城墙档案' : '新建城墙档案'" width="560" :close-on-click-modal="false">
      <el-form label-width="100px">
        <el-form-item label="编码" required><el-input v-model="segmentForm.code" maxlength="32" /></el-form-item>
        <el-form-item label="区段名称" required><el-input v-model="segmentForm.name" maxlength="128" /></el-form-item>
        <el-form-item label="档案说明"><el-input v-model="segmentForm.description" type="textarea" :rows="5" maxlength="512" placeholder="位置范围、可见特征及巡检注意事项" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="segmentForm.active" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="segmentVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitSegment">保存</el-button></template>
    </el-dialog>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchPoints, fetchSegments, createPoint, updatePoint, saveSegment, type InspectionPoint, type WallSegment } from '@/shared/api/site'
const points = ref<InspectionPoint[]>([])
const segments = ref<WallSegment[]>([])
const keyword = ref('')
const segmentFilter = ref<number>()
const activeFilter = ref('all')
const pointUsable = (point: InspectionPoint) => point.active && !!segments.value.find(segment => segment.id === point.segment_id)?.active
const usablePoints = computed(() => points.value.filter(pointUsable).length)
const matches = (...values: (string | null)[]) => values.some(value => (value ?? '').toLocaleLowerCase().includes(keyword.value.trim().toLocaleLowerCase()))
const filteredPoints = computed(() => points.value.filter(point =>
  (!segmentFilter.value || point.segment_id === segmentFilter.value) &&
  (activeFilter.value === 'all' || pointUsable(point) === (activeFilter.value === 'active')) &&
  matches(point.code, point.name, point.location_hint)))
const filteredSegments = computed(() => segments.value.filter(segment =>
  (!segmentFilter.value || segment.id === segmentFilter.value) &&
  (activeFilter.value === 'all' || segment.active === (activeFilter.value === 'active')) &&
  matches(segment.code, segment.name, segment.description)))
const loading = ref(false)
const saving = ref(false)
const pointVisible = ref(false)
const segmentVisible = ref(false)
const pointId = ref<number | null>(null)
const segmentId = ref<number | null>(null)
const pointForm = reactive({ segment_id: 0, code: '', name: '', latitude: null as number | null, longitude: null as number | null, radius_m: 50, location_hint: '', active: true })
const segmentForm = reactive({ code: '', name: '', description: '', active: true })
async function load() {
  loading.value = true
  try { [points.value, segments.value] = await Promise.all([fetchPoints(true), fetchSegments(true)]) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '加载档案失败') }
  finally { loading.value = false }
}
function editPoint(point?: InspectionPoint) {
  if (!segments.value.length) { ElMessage.warning('请先建立城墙区段'); return }
  pointId.value = point?.id ?? null
  Object.assign(pointForm, point ? { ...point, radius_m: point.radius_m ?? 50, location_hint: point.location_hint ?? '' } : { segment_id: segments.value[0]?.id ?? 0, code: '', name: '', latitude: null, longitude: null, radius_m: 50, location_hint: '', active: true })
  pointVisible.value = true
}
function editSegment(segment?: WallSegment) {
  segmentId.value = segment?.id ?? null
  Object.assign(segmentForm, { code: segment?.code ?? '', name: segment?.name ?? '', description: segment?.description ?? '', active: segment?.active ?? true })
  segmentVisible.value = true
}
async function submitPoint() {
  if (saving.value) return
  if (!pointForm.segment_id || !pointForm.code.trim() || !pointForm.name.trim()) { ElMessage.warning('请填写区段、编码和名称'); return }
  const { latitude, longitude } = pointForm
  if (latitude == null || longitude == null || !Number.isFinite(latitude) || !Number.isFinite(longitude)) { ElMessage.warning('请填写有效的经纬度'); return }
  const payload = { ...pointForm, latitude, longitude }
  saving.value = true
  try {
    if (pointId.value) await updatePoint(pointId.value, payload)
    else await createPoint(payload)
    pointVisible.value = false; ElMessage.success('点位已保存'); await load()
  } catch (e) { ElMessage.error(e instanceof Error ? e.message : '保存失败') }
  finally { saving.value = false }
}
async function submitSegment() {
  if (saving.value) return
  if (!segmentForm.code.trim() || !segmentForm.name.trim()) { ElMessage.warning('请填写编码和名称'); return }
  saving.value = true
  try { await saveSegment(segmentId.value, segmentForm); segmentVisible.value = false; ElMessage.success('档案已保存'); await load() }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '保存失败') }
  finally { saving.value = false }
}
onMounted(load)
</script>
<style scoped>
.page { background: white; padding: 20px; border-radius: 12px; }
.toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin-bottom: 16px; }
.archive-summary { display: flex; gap: 8px; align-items: baseline; padding: 12px 0 20px; font-size: 13px; color: var(--text-muted); }
.archive-summary strong { font-size: 22px; font-weight: 500; color: var(--brand); }
.archive-summary span { margin: 0 8px; }
</style>
