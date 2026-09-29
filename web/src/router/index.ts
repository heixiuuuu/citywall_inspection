import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

// 路由表按模块拆分（taskRoutes / reviewRoutes / …），入口显式合并并按需加载。
// 菜单基于登录用户权限生成；前端守卫只隐藏不可操作页面，不能代替 Go 的授权。
const taskRoutes: RouteRecordRaw[] = [
  {
    path: '/tasks',
    name: 'tasks',
    component: () => import('@/pages/tasks/index.vue'),
    meta: { title: '任务管理' },
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/tasks' },
    ...taskRoutes,
    { path: '/:pathMatch(.*)*', redirect: '/tasks' },
  ],
})
