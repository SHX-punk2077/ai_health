import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { getToken } from '@/utils/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/login/LoginView.vue'),
    meta: { public: true, title: '登录' },
  },
  {
    path: '/',
    component: () => import('@/layouts/DefaultLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/views/dashboard/DashboardView.vue'),
        meta: { title: '概览' },
      },
      {
        path: 'health/profile',
        name: 'health-profile',
        component: () => import('@/views/health/ProfileView.vue'),
        meta: { title: '健康档案' },
      },
      {
        path: 'ai/chat',
        name: 'ai-chat',
        component: () => import('@/views/ai/ChatView.vue'),
        meta: { title: 'AI 健康问答' },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/error/NotFound.vue'),
    meta: { public: true, title: '404' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 全局守卫：非公开页必须已登录
router.beforeEach((to) => {
  document.title = (to.meta.title as string) || 'AI 健康管理系统'
  if (to.meta.public) return true
  if (!getToken()?.accessToken) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
