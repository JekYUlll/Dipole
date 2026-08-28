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
      path: '/agent/tasks/:taskId/approval',
      name: 'agent-task-approval',
      component: () => import('@/views/AgentApprovalView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => import.meta.env.VITE_AGENT_APPROVAL_ENABLED === 'true'
        ? true
        : { name: 'chat' },
    },
    {
      path: '/agent/tasks/:taskId/timeline',
      name: 'agent-task-timeline',
      component: () => import('@/views/AgentTaskTimelineView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => import.meta.env.VITE_AGENT_TIMELINE_ENABLED === 'true'
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
    {
      path: '/agent/memories',
      name: 'agent-memories',
      component: () => import('@/views/AgentMemoriesView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => import.meta.env.VITE_AGENT_MEMORIES_ENABLED === 'true'
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
