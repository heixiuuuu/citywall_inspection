import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

/** 角色元数据。 */
export interface RouteMeta {
  title?: string
  roles?: string[]
}

// 登录页（无需鉴权）。
const loginRoute: RouteRecordRaw = {
  path: '/login',
  name: 'login',
  component: () => import('@/pages/login/index.vue'),
  meta: { title: '登录' },
}

// 管理端布局 + 各功能页。
const adminRoutes: RouteRecordRaw = {
  path: '/',
  component: () => import('@/layouts/AdminLayout.vue'),
  redirect: '/tasks',
  children: [
    {
      path: 'tasks',
      name: 'tasks',
      component: () => import('@/pages/tasks/index.vue'),
      meta: { title: '任务管理', roles: ['ADMIN', 'REVIEWER', 'INSPECTOR'] },
    },
    {
      path: 'reviews',
      name: 'reviews',
      component: () => import('@/pages/reviews/index.vue'),
      meta: { title: '审核管理', roles: ['REVIEWER', 'ADMIN'] },
    },
    {
      path: 'issues',
      name: 'issues',
      component: () => import('@/pages/issues/index.vue'),
      meta: { title: '问题台账', roles: ['ADMIN', 'REVIEWER', 'INSPECTOR'] },
    },
    {
      path: 'records',
      name: 'records',
      component: () => import('@/pages/records/index.vue'),
      meta: { title: '记录检索', roles: ['ADMIN', 'REVIEWER'] },
    },
    {
      path: 'reports',
      name: 'reports',
      component: () => import('@/pages/reports/index.vue'),
      meta: { title: '报告中心', roles: ['ADMIN', 'REVIEWER'] },
    },
    {
      path: 'points',
      name: 'points',
      component: () => import('@/pages/points/index.vue'),
      meta: { title: '城墙档案', roles: ['ADMIN'] },
    },
    {
      path: 'audit',
      name: 'audit',
      component: () => import('@/pages/audit/index.vue'),
      meta: { title: '审计日志', roles: ['ADMIN'] },
    },
  ],
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [loginRoute, adminRoutes, { path: '/:pathMatch(.*)*', redirect: '/tasks' }],
})

/** 路由守卫：登录态 + 角色；前端守卫只隐藏不可操作页面，不能代替 Go 的授权。 */
router.beforeEach((to) => {
  const token = localStorage.getItem('city_wall_token')
  const roles: string[] = JSON.parse(localStorage.getItem('city_wall_roles') ?? '[]')
  if (!token) {
    return to.name === 'login' ? true : { name: 'login', query: { redirect: to.fullPath } }
  }
  const meta = to.meta as RouteMeta
  if (meta.roles && !meta.roles.some((r) => roles.includes(r))) {
    return { name: 'tasks' }
  }
  return true
})
