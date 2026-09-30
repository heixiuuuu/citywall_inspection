<template>
  <div class="audit-page">
    <el-card shadow="never">
      <div class="filters">
        <el-select v-model="filters.entity_type" clearable placeholder="实体类型" style="width: 150px" @change="reload">
          <el-option label="任务 task" value="task" />
          <el-option label="记录 observation" value="observation" />
        </el-select>
        <el-input v-model="filters.entity_id" placeholder="实体 ID" style="width: 140px" @keyup.enter="reload" />
        <el-input v-model="filters.actor_id" placeholder="操作人 ID" style="width: 140px" @keyup.enter="reload" />
        <el-button type="primary" @click="reload">查询</el-button>
        <el-button @click="reset">重置</el-button>
      </div>

      <el-table :data="rows" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="entity_type" label="实体" width="110" />
        <el-table-column prop="entity_id" label="实体ID" width="80" />
        <el-table-column prop="action" label="操作" width="110" />
        <el-table-column prop="actor_id" label="操作人" width="80" />
        <el-table-column label="状态变化" min-width="150">
          <template #default="{ row }">
            <span v-if="row.old_status || row.new_status">
              {{ row.old_status || '—' }} → {{ row.new_status || '—' }}
            </span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因/说明" min-width="180" show-overflow-tooltip />
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next, sizes"
          :total="total"
          :page-size="filters.page_size ?? 50"
          :current-page="filters.page ?? 1"
          :page-sizes="[20, 50, 100]"
          @current-change="(p: number) => go(1, p)"
          @size-change="(s: number) => go(s, 1)"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchAuditEvents, type AuditEvent } from '@/shared/api/audit'

const loading = ref(false)
const rows = ref<AuditEvent[]>([])
const total = ref(0)
const filters = reactive<{ entity_type?: string; entity_id?: string; actor_id?: string; page?: number; page_size?: number }>({
  entity_type: undefined,
  entity_id: '',
  actor_id: '',
  page: 1,
  page_size: 50,
})

function formatTime(s: string): string {
  if (!s) return ''
  return new Date(s).toLocaleString('zh-CN', { hour12: false })
}

async function load() {
  loading.value = true
  try {
    const res = await fetchAuditEvents({
      entity_type: filters.entity_type,
      entity_id: filters.entity_id || undefined,
      actor_id: filters.actor_id || undefined,
      page: filters.page,
      page_size: filters.page_size,
    })
    rows.value = res.events
    total.value = Number(res.total)
  } catch (e) {
    ElMessage.error((e as Error).message || '审计事件加载失败')
  } finally {
    loading.value = false
  }
}

function reload() {
  filters.page = 1
  load()
}

function go(pageSize: number, page: number) {
  filters.page_size = pageSize
  filters.page = page
  load()
}

function reset() {
  filters.entity_type = undefined
  filters.entity_id = ''
  filters.actor_id = ''
  filters.page = 1
  load()
}

onMounted(load)
</script>

<style scoped>
.filters {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}
</style>
