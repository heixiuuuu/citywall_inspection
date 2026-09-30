<template>
  <div class="page">
    <div class="toolbar">
      <el-button type="primary" @click="createVisible = true">新建点位</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="points" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="code" label="编码" width="130" />
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column label="区段" width="130">
        <template #default="{ row }">{{ segmentName(row.segment_id) }}</template>
      </el-table-column>
      <el-table-column label="坐标" min-width="180">
        <template #default="{ row }">{{ row.latitude?.toFixed(6) }}, {{ row.longitude?.toFixed(6) }}</template>
      </el-table-column>
      <el-table-column prop="location_hint" label="位置提示" min-width="160" />
      <el-table-column label="半径(m)" width="90">
        <template #default="{ row }">{{ row.radius_m ?? 50 }}</template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="createVisible" title="新建巡检点位" width="520">
      <el-form :model="form" label-width="100px">
        <el-form-item label="区段" required>
          <el-select v-model="form.segment_id" placeholder="选择区段" style="width: 100%">
            <el-option v-for="s in segments" :key="s.id" :label="`${s.code} ${s.name}`" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="编码" required>
          <el-input v-model="form.code" placeholder="如 P-EAST-03" />
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如 东城墙 3 号点" />
        </el-form-item>
        <el-form-item label="纬度" required>
          <el-input-number v-model="form.latitude" :precision="7" :step="0.0001" />
        </el-form-item>
        <el-form-item label="经度" required>
          <el-input-number v-model="form.longitude" :precision="7" :step="0.0001" />
        </el-form-item>
        <el-form-item label="半径(m)">
          <el-input-number v-model="form.radius_m" :min="1" :max="500" />
        </el-form-item>
        <el-form-item label="位置提示">
          <el-input v-model="form.location_hint" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="create">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchSegments, fetchPoints, createPoint, type WallSegment } from '@/shared/api/site'

const points = ref<any[]>([])
const segments = ref<WallSegment[]>([])
const loading = ref(false)
const createVisible = ref(false)
const creating = ref(false)
const form = reactive({
  segment_id: 0,
  code: '',
  name: '',
  latitude: 28.84,
  longitude: 121.13,
  radius_m: 50,
  location_hint: '',
})

onMounted(() => {
  load()
  loadSegments()
})

async function load() {
  loading.value = true
  try {
    points.value = await fetchPoints()
  } catch {
    ElMessage.error('加载点位失败')
  } finally {
    loading.value = false
  }
}

async function loadSegments() {
  try {
    segments.value = await fetchSegments()
  } catch {
    ElMessage.error('加载区段失败')
  }
}

function segmentName(id: number) {
  const s = segments.value.find((x) => x.id === id)
  return s ? `${s.code} ${s.name}` : `#${id}`
}

async function create() {
  if (!form.segment_id || !form.code || !form.name) {
    ElMessage.warning('区段/编码/名称必填')
    return
  }
  creating.value = true
  try {
    await createPoint({ ...form, radius_m: form.radius_m })
    ElMessage.success('点位已创建')
    createVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
  } finally {
    creating.value = false
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
  gap: 10px;
  margin-bottom: 14px;
}
</style>
