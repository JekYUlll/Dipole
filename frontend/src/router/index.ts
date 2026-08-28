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
      path: '/agent/tasks/:taskId/input',
      name: 'agent-task-input',
      component: () => import('@/views/AgentElicitationView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => import.meta.env.VITE_AGENT_ELICITATION_ENABLED === 'true'
        ? true
        : { name: 'chat' },
    },
    {
      path: '/agent/subscriptions',
      name: 'agent-subscriptions',
      component: () => import('@/views/AgentSubscriptionsView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => import.meta.env.VITE_AGENT_SUBSCRIPTIONS_ENABLED === 'true'
        ? true
        : { name: 'chat' },
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
