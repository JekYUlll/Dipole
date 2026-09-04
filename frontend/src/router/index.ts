import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory('/app/'),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
    },
    {
      path: '/',
      name: 'chat',
      component: () => import('@/views/ChatView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/contacts',
      name: 'contacts',
      component: () => import('@/views/ContactDirectoryView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/groups',
      name: 'groups',
      component: () => import('@/views/GroupDirectoryView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/files',
      name: 'files',
      component: () => import('@/views/FileDirectoryView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/devices',
      name: 'devices',
      component: () => import('@/views/DeviceSecurityView.vue'),
      meta: { requiresAuth: true },
    },
    {
      // Settings 不再作为独立页面。历史链接 /settings 一律回到 Chat,
      // 并通过 query 触发弹窗;由 ChatView 监听 `settings=1` 打开 SettingsDialog。
      path: '/settings',
      name: 'settings',
      redirect: { path: '/', query: { settings: '1' } },
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.token) {
    return { name: 'login' }
  }
  if (to.name === 'login' && auth.token) {
    return { name: 'chat' }
  }
})

export default router
