<template>
  <el-container class="layout">
    <el-aside width="232px" class="aside">
      <router-link :to="canReview ? '/overview' : '/tasks'" class="brand"><span class="brand-mark"><AppIcon name="wall" /></span><span><strong>城墙巡检</strong><small>台州府城墙 · 日常保护</small></span></router-link>
      <el-menu :default-active="$route.path" router class="menu">
        <template v-for="group in navigation" :key="group.label"><p class="nav-group" role="presentation">{{ group.label }}</p><el-menu-item v-for="item in group.items" :key="item.path" :index="item.path"><AppIcon :name="item.icon" /><span>{{ item.title }}</span></el-menu-item></template>
      </el-menu>
      <div class="aside-note"><span class="note-dot" />采集有据 · 复核留痕</div>
    </el-aside>
    <el-container class="content">
      <el-header class="header">
        <div class="breadcrumb"><span>巡检管理</span><span aria-hidden="true">/</span><strong>{{ $route.meta.title || '' }}</strong></div>
        <div class="user">
          <span class="user-avatar" aria-hidden="true">{{ userName.slice(0, 1) }}</span><span>{{ userName }}</span><span class="role-label">{{ roleLabel }}</span>
          <el-button text @click="logout">退出登录</el-button>
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
import AppIcon from '@/shared/ui/AppIcon.vue'

const router = useRouter()

const roles: string[] = JSON.parse(localStorage.getItem('city_wall_roles') ?? '[]')
const user = JSON.parse(localStorage.getItem('city_wall_user') ?? 'null')

const canReview = computed(() => roles.includes('REVIEWER') || roles.includes('ADMIN'))
const isAdmin = computed(() => roles.includes('ADMIN'))
const userName = computed(() => (user ? user.display_name : ''))
const roleLabel = computed(() => isAdmin.value ? '管理员' : canReview.value ? '审核员' : '巡检员')
const navigation = computed(() => [
  { label: '巡检与跟进', items: [
    ...(canReview.value ? [{ path: '/overview', title: '巡检工作台', icon: 'overview' }] : []),
    { path: '/tasks', title: '任务管理', icon: 'tasks' },
    ...(canReview.value ? [{ path: '/reviews', title: '审核管理', icon: 'reviews' }] : []),
    { path: '/issues', title: '问题台账', icon: 'issues' },
  ] },
  ...(canReview.value ? [{ label: '证据与成果', items: [{ path: '/records', title: '记录检索', icon: 'records' }, { path: '/reports', title: '报告中心', icon: 'reports' }] }] : []),
  ...(isAdmin.value ? [{ label: '基础管理', items: [{ path: '/points', title: '城墙档案', icon: 'points' }, { path: '/audit', title: '审计日志', icon: 'audit' }] }] : []),
])

function logout() {
  localStorage.removeItem('city_wall_token')
  localStorage.removeItem('city_wall_roles')
  localStorage.removeItem('city_wall_user')
  router.replace('/login')
}
</script>

<style scoped>
.layout { min-height: 100vh; align-items: flex-start; }
.content { min-height: 100vh; width: 0; flex: 1; }
.aside { background: var(--brand-ink); height: 100vh; position: sticky; top: 0; display: flex; flex-direction: column; }
.brand { display: flex; align-items: center; gap: 12px; padding: 28px 22px 20px; color: var(--nav-text); text-decoration: none; }
.brand-mark { --icon-size: 28px; width: 42px; height: 44px; display: grid; place-items: center; background: var(--nav-selected); color: var(--brand-accent); border-radius: var(--radius-control); }
.brand strong { display: block; font-size: 19px; font-weight: 600; letter-spacing: .12em; }
.brand small { display: block; margin-top: 3px; font-size: 11px; color: var(--nav-muted); }
.menu { border: none; background: transparent; padding: 0 12px; flex: 1; overflow-y: auto; }
.nav-group { font-size: 11px; letter-spacing: .08em; color: var(--nav-muted); margin: 22px 12px 9px; }
.menu :deep(.el-menu-item) { color: var(--nav-text); gap: 12px; height: 46px; border-radius: var(--radius-control); margin-bottom: 4px; }
.menu :deep(.el-menu-item:hover) { background: var(--brand); }
.menu :deep(.el-menu-item.is-active) { background: var(--nav-selected); color: var(--surface); box-shadow: inset 3px 0 var(--brand-accent); font-weight: 500; }
.aside-note { padding: 24px; color: var(--nav-muted); font-size: 11px; display: flex; align-items: center; gap: var(--space-tight); }
.note-dot { width: 5px; height: 5px; border-radius: 50%; background: var(--brand-accent); }
.header { background: var(--surface); display: flex; align-items: center; justify-content: space-between; gap: var(--space-content); }
.breadcrumb, .user { display: flex; align-items: center; gap: 12px; font-size: var(--font-small); color: var(--text-muted); }
.breadcrumb strong { color: var(--text-strong); font-weight: 500; }
.user-avatar { width: 30px; height: 30px; background: var(--surface-sunken); color: var(--brand); display: grid; place-items: center; border-radius: 50%; }
.role-label { background: var(--surface-subtle); border-radius: var(--radius-control); font-size: 11px; padding: 2px 8px; }
.main { background: var(--app-background); padding: var(--space-section); }
@media (max-width: 760px) {
  .layout { flex-direction: column; }
  .content { width: 100%; }
  .aside { width: 100% !important; height: auto; position: static; }
  .brand { padding: 14px 16px 8px; }
  .brand-mark { width: 34px; height: 36px; }
  .brand strong { font-size: 16px; }
  .brand small, .nav-group, .aside-note, .role-label, .breadcrumb > span { display: none; }
  .menu { display: flex; overflow-x: auto; padding: 6px 8px; }
  .menu :deep(.el-menu-item) { height: 42px; flex-shrink: 0; padding: 0 12px; gap: 7px; }
  .menu :deep(.el-menu-item.is-active) { box-shadow: inset 0 -2px var(--brand-accent); }
  .main { padding: var(--space-content); }
  .header { padding: 0 var(--space-content); }
  .user { gap: 8px; }
}
</style>
