import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { agentFlags, agentTaskCreatePageEnabled, agentTaskInboxEnabled } from '@/config/agentFlags'

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
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/SettingsView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/agent/tasks',
      name: 'agent-task-inbox',
      component: () => import('@/views/AgentTaskInboxView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentTaskInboxEnabled || { name: 'chat' },
    },
    {
      path: '/agent/tasks/new',
      name: 'agent-task-create',
      component: () => import('@/views/AgentTaskCreateView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentTaskCreatePageEnabled || { name: 'chat' },
    },
    {
      path: '/agent/tasks/:taskId/input',
      name: 'agent-task-input',
      component: () => import('@/views/AgentElicitationView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.elicitation || { name: 'chat' },
    },
    {
      path: '/agent/tasks/:taskId/approval',
      name: 'agent-task-approval',
      component: () => import('@/views/AgentApprovalView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.approval || { name: 'chat' },
    },
    {
      path: '/agent/tasks/:taskId/timeline',
      name: 'agent-task-timeline',
      component: () => import('@/views/AgentTaskTimelineView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.timeline || { name: 'chat' },
    },
    {
      path: '/agent/artifacts',
      name: 'agent-artifact-inbox',
      component: () => import('@/views/AgentArtifactInboxView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.artifacts || { name: 'chat' },
    },
    {
      path: '/agent/artifacts/:artifactId',
      name: 'agent-artifact',
      component: () => import('@/views/AgentArtifactView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.artifacts || { name: 'chat' },
    },
    {
      path: '/agent/subscriptions',
      name: 'agent-subscriptions',
      component: () => import('@/views/AgentSubscriptionsView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.subscriptions || { name: 'chat' },
    },
    {
      path: '/agent/definitions',
      name: 'agent-definitions',
      component: () => import('@/views/AgentDefinitionsView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.definitions || { name: 'chat' },
    },
    {
      path: '/agent/memories',
      name: 'agent-memories',
      component: () => import('@/views/AgentMemoriesView.vue'),
      meta: { requiresAuth: true },
      beforeEnter: () => agentFlags.memories || { name: 'chat' },
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
