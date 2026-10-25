<template>
  <div class="page" v-loading="loading">
    <div class="toolbar">
      <el-button type="primary" @click="editPoint()">新建点位</el-button>
      <el-button @click="editSegment()">新建城墙区段</el-button>
      <el-button @click="load">刷新</el-button>
    </div>
    <el-tabs>
      <el-tab-pane label="巡检点位">
        <el-table :data="points" stripe>
          <el-table-column prop="code" label="编码" width="150" />
          <el-table-column prop="name" label="名称" min-width="150" />
          <el-table-column label="区段" min-width="140"><template #default="{row}">{{ segments.find(s => s.id === row.segment_id)?.name || row.segment_id }}</template></el-table-column>
          <el-table-column label="经纬度" min-width="200"><template #default="{row}">{{ row.latitude.toFixed(6) }}, {{ row.longitude.toFixed(6) }}</template></el-table-column>
          <el-table-column prop="radius_m" label="定位半径(m)" width="110" />
          <el-table-column prop="location_hint" label="位置提示" min-width="150" />
          <el-table-column label="状态" width="90"><template #default="{row}"><el-tag :type="row.active ? 'success' : 'info'">{{ row.active ? '启用' : '停用' }}</el-tag></template></el-table-column>
          <el-table-column label="操作" width="80" fixed="right"><template #default="{row}"><el-button link type="primary" @click="editPoint(row)">编辑</el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="城墙档案">
        <el-table :data="segments" stripe>
          <el-table-column prop="code" label="编码" width="150" />
          <el-table-column prop="name" label="区段名称" min-width="150" />
          <el-table-column prop="description" label="档案说明" min-width="280" />
          <el-table-column label="状态" width="90"><template #default="{row}">{{ row.active ? '启用' : '停用' }}</template></el-table-column>
          <el-table-column label="操作" width="80"><template #default="{row}"><el-button link type="primary" @click="editSegment(row)">编辑</el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
    <el-dialog v-model="pointVisible" :title="pointId ? '编辑点位' : '新建点位'" width="560">
      <el-form label-width="100px">
        <el-form-item label="区段" required><el-select v-model="pointForm.segment_id"><el-option v-for="s in segments" :key="s.id" :label="s.name" :value="s.id" /></el-select></el-form-item>
        <el-form-item label="编码" required><el-input v-model="pointForm.code" maxlength="64" /></el-form-item>
        <el-form-item label="名称" required><el-input v-model="pointForm.name" maxlength="128" /></el-form-item>
        <el-form-item label="纬度"><el-input-number v-model="pointForm.latitude" :min="-90" :max="90" :precision="7" /></el-form-item>
        <el-form-item label="经度"><el-input-number v-model="pointForm.longitude" :min="-180" :max="180" :precision="7" /></el-form-item>
        <el-form-item label="定位半径(m)"><el-input-number v-model="pointForm.radius_m" :min="1" :max="500" /></el-form-item>
        <el-form-item label="位置提示"><el-input v-model="pointForm.location_hint" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="pointForm.active" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="pointVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitPoint">保存</el-button></template>
    </el-dialog>
    <el-dialog v-model="segmentVisible" :title="segmentId ? '编辑城墙档案' : '新建城墙档案'" width="560">
      <el-form label-width="100px">
        <el-form-item label="编码" required><el-input v-model="segmentForm.code" maxlength="64" /></el-form-item>
        <el-form-item label="区段名称" required><el-input v-model="segmentForm.name" maxlength="128" /></el-form-item>
        <el-form-item label="档案说明"><el-input v-model="segmentForm.description" type="textarea" :rows="5" placeholder="位置范围、可见特征及巡检注意事项" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="segmentForm.active" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="segmentVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitSegment">保存</el-button></template>
    </el-dialog>
  </div>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchPoints, fetchSegments, createPoint, updatePoint, saveSegment, type InspectionPoint, type WallSegment } from '@/shared/api/site'
const points = ref<InspectionPoint[]>([])
const segments = ref<WallSegment[]>([])
const loading = ref(false)
const saving = ref(false)
const pointVisible = ref(false)
const segmentVisible = ref(false)
const pointId = ref<number | null>(null)
const segmentId = ref<number | null>(null)
const pointForm = reactive({ segment_id: 0, code: '', name: '', latitude: 28.84, longitude: 121.13, radius_m: 50, location_hint: '', active: true })
const segmentForm = reactive({ code: '', name: '', description: '', active: true })
async function load() {
  loading.value = true
  try { [points.value, segments.value] = await Promise.all([fetchPoints(true), fetchSegments(true)]) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '加载档案失败') }
  finally { loading.value = false }
}
function editPoint(point?: InspectionPoint) {
  pointId.value = point?.id ?? null
  Object.assign(pointForm, point ? { ...point, radius_m: point.radius_m ?? 50, location_hint: point.location_hint ?? '' } : { segment_id: segments.value[0]?.id ?? 0, code: '', name: '', latitude: 28.84, longitude: 121.13, radius_m: 50, location_hint: '', active: true })
  pointVisible.value = true
}
function editSegment(segment?: WallSegment) {
  segmentId.value = segment?.id ?? null
  Object.assign(segmentForm, { code: segment?.code ?? '', name: segment?.name ?? '', description: segment?.description ?? '', active: segment?.active ?? true })
  segmentVisible.value = true
}
async function submitPoint() {
  if (!pointForm.segment_id || !pointForm.code.trim() || !pointForm.name.trim()) { ElMessage.warning('请填写区段、编码和名称'); return }
  saving.value = true
  try {
    if (pointId.value) await updatePoint(pointId.value, pointForm)
    else await createPoint(pointForm)
    pointVisible.value = false; ElMessage.success('点位已保存'); await load()
  } catch (e) { ElMessage.error(e instanceof Error ? e.message : '保存失败') }
  finally { saving.value = false }
}
async function submitSegment() {
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
.toolbar { display: flex; gap: 10px; margin-bottom: 16px; }
</style>
