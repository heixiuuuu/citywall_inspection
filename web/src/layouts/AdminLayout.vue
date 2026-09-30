<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="brand">古城墙病害巡检</div>
      <el-menu :default-active="$route.path" router class="menu">
        <el-menu-item index="/tasks">
          <span>任务管理</span>
        </el-menu-item>
        <el-menu-item v-if="canReview" index="/reviews">
          <span>审核管理</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/points">
          <span>点位管理</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/audit">
          <span>审计日志</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span class="title">{{ $route.meta.title || '' }}</span>
        <div class="user">
          <span>{{ userName }}</span>
          <el-button link type="primary" @click="logout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

const roles: string[] = JSON.parse(localStorage.getItem('city_wall_roles') ?? '[]')
const user = JSON.parse(localStorage.getItem('city_wall_user') ?? 'null')

const canReview = computed(() => roles.includes('REVIEWER') || roles.includes('ADMIN'))
const isAdmin = computed(() => roles.includes('ADMIN'))
const userName = computed(() => (user ? user.display_name : ''))

function logout() {
  localStorage.removeItem('city_wall_token')
  localStorage.removeItem('city_wall_roles')
  localStorage.removeItem('city_wall_user')
  router.replace('/login')
}
</script>

<style scoped>
.layout {
  min-height: 100vh;
}
.aside {
  background: #25584f;
}
.brand {
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  padding: 20px 20px 14px;
}
.menu {
  border-right: none;
  background: transparent;
}
.menu :deep(.el-menu-item) {
  color: rgba(255, 255, 255, 0.85);
}
.menu :deep(.el-menu-item.is-active) {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}
.header {
  background: #fff;
  border-bottom: 1px solid #e6e1d5;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.title {
  font-size: 16px;
  font-weight: 600;
  color: #2b2b2b;
}
.user {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  color: #6b6b6b;
}
.main {
  background: #f5f1e8;
  padding: 20px;
}
</style>
