<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="brand">古城墙病害巡检</div>
      <el-menu :default-active="$route.path" router class="menu">
        <el-menu-item v-if="canReview" index="/overview"><span>巡检工作台</span></el-menu-item>
        <el-menu-item index="/tasks">
          <span>任务管理</span>
        </el-menu-item>
        <el-menu-item v-if="canReview" index="/reviews">
          <span>审核管理</span>
        </el-menu-item>
        <el-menu-item index="/issues">
          <span>问题台账</span>
        </el-menu-item>
        <el-menu-item v-if="canReview" index="/records">
          <span>记录检索</span>
        </el-menu-item>
        <el-menu-item v-if="canReview" index="/reports">
          <span>报告中心</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/points">
          <span>城墙档案</span>
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
  background: var(--app-background);
  padding: var(--space-section);
}
@media (max-width: 760px) {
  .layout { flex-direction: column; }
  .aside { width: 100% !important; }
  .brand { padding: 12px 16px 4px; }
  .menu { display: flex; overflow-x: auto; }
  .menu :deep(.el-menu-item) { height: 44px; flex-shrink: 0; padding: 0 16px; }
  .main { padding: var(--space-content); }
}
</style>
