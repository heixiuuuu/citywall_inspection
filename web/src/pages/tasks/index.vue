<template>
  <el-main>
    <el-card>
      <template #header>任务管理（初始化骨架）</template>
      <el-button type="primary" :loading="loading" @click="load">刷新</el-button>
      <el-empty v-if="!loading && tasks.length === 0" description="暂无任务" />
      <el-table v-else :data="tasks" border>
        <el-table-column prop="id" label="ID" width="100" />
        <el-table-column prop="title" label="标题" />
        <el-table-column prop="status" label="状态" width="120" />
      </el-table>
    </el-card>
  </el-main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { fetchTasks, type Task } from '@/shared/api/task'

const tasks = ref<Task[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    tasks.value = await fetchTasks()
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
