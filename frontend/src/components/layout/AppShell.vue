<script setup lang="ts">
// AppShell — top-level chrome shared by Chat.
//
// Layout:
//   [ Top bar 48px ]           (brand · Chat 徽标 · Agent 切换 · 设置 · 头像)
//   [ Main slot (fills) | Agent Drawer slot ]
//   [ Status bar 28px ]
//
// 顶部只保留一个"设置"入口 —— 齿轮按钮打开 SettingsDialog（父组件
// 用 v-model:open-settings 监听）；不再有 Settings tab 或多余的搜索按钮，
// 主界面的搜索属于 Chat 面板本身。

import { computed } from 'vue'
import { RouterLink, useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { IconCpu, IconSettings } from '@/components/icons'

export type WorkspaceKey = 'chat' | 'settings'

const props = withDefaults(defineProps<{
  /** Which top-level workspace is currently focused. */
  activeWorkspace?: WorkspaceKey
  /** Highlight the Agent toggle (Drawer open). */
  agentActive?: boolean
  /** Pending Agent input/approval count; renders as red badge when > 0. */
  agentPending?: number
  /** Bottom status bar caption (env, connection). */
  statusText?: string
  /** Show the bottom status bar. Default true. */
  showStatusBar?: boolean
}>(), { activeWorkspace: 'chat', agentActive: false, agentPending: 0, showStatusBar: true })

const emit = defineEmits<{
  (e: 'toggle-agent'): void
  (e: 'open-settings'): void
}>()

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const initials = computed(() => {
  const name = auth.currentUser?.nickname ?? auth.currentUser?.uuid ?? ''
  if (!name) return '·'
  return name.trim().slice(0, 2).toUpperCase()
})

function onAgentToggle() {
  const isOpen = route.query.agent === '1'
  const query = { ...route.query }
  if (isOpen) {
    delete query.agent
    delete query.view
    delete query.task
    delete query.artifact
    delete query.panel
    delete query.drawer
  } else {
    query.agent = '1'
    if (!query.view) query.view = 'live'
  }
  router.replace({ query })
  emit('toggle-agent')
}

function onOpenSettings() {
  emit('open-settings')
}
</script>

<template>
  <div class="app-shell">
    <!-- Top bar 48px -->
    <header class="app-shell__topbar" role="banner">
      <RouterLink to="/" class="app-shell__brand">
        <span class="app-shell__brand-dot" aria-hidden="true" />
        <span class="app-shell__brand-word">DIPOLE</span>
      </RouterLink>

      <!-- Chat 是唯一的主活动 tab；不再重复放 Settings。 -->
      <nav class="app-shell__tabs" aria-label="workspaces">
        <RouterLink
          to="/"
          class="app-shell__tab"
          :class="{ 'app-shell__tab--active': props.activeWorkspace === 'chat' }"
        >Chat</RouterLink>
      </nav>

      <div class="app-shell__spacer" aria-hidden="true" />

      <div class="app-shell__right">
        <!-- Agent Drawer toggle: the ONLY drawer entry. -->
        <button
          type="button"
          class="app-shell__agent-toggle"
          :class="{ 'app-shell__agent-toggle--active': props.agentActive }"
          :aria-pressed="props.agentActive"
          data-agent-toggle
          title="Agent"
          aria-label="切换 Agent 抽屉"
          @click="onAgentToggle"
        >
          <IconCpu :size="16" />
          <span
            v-if="props.agentPending > 0"
            class="app-shell__agent-badge"
            :aria-label="`待处理 ${props.agentPending} 项`"
          >{{ props.agentPending > 99 ? '99+' : props.agentPending }}</span>
        </button>

        <!-- Settings 入口：唯一，弹 SettingsDialog。 -->
        <button
          type="button"
          class="app-shell__icon-btn"
          :class="{ 'app-shell__icon-btn--active': props.activeWorkspace === 'settings' }"
          data-open-settings
          title="设置"
          aria-label="打开设置"
          @click="onOpenSettings"
        >
          <IconSettings :size="16" />
        </button>

        <button
          type="button"
          class="app-shell__avatar"
          :title="auth.currentUser?.nickname ?? '账户'"
          aria-label="账户菜单"
          @click="onOpenSettings"
        >
          <span>{{ initials }}</span>
        </button>
      </div>
    </header>

    <!-- Main + optional Agent Drawer -->
    <div class="app-shell__body">
      <main class="app-shell__main"><slot /></main>
      <slot name="agent-drawer" />
    </div>

    <!-- Status bar 28px -->
    <footer
      v-if="props.showStatusBar"
      class="app-shell__statusbar"
      role="contentinfo"
    >
      <span class="app-shell__status-text">{{ props.statusText ?? 'DIPOLE' }}</span>
      <span class="app-shell__spacer" aria-hidden="true" />
      <slot name="status-right" />
    </footer>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  background: var(--dp-canvas);
  color: var(--dp-ink);
  font-family: var(--dp-font-body);
}

/* Top bar --------------------------------------------------------------- */
.app-shell__topbar {
  display: flex;
  align-items: center;
  gap: 32px;
  height: 48px;
  padding: 0 20px;
  background: var(--dp-rail);
  color: var(--dp-text-inverse);
  flex-shrink: 0;
  border-radius: 0;
}

.app-shell__brand {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--dp-text-inverse);
  text-decoration: none;
  font: 700 15px var(--dp-font-display);
  letter-spacing: 0.12em;
}
.app-shell__brand-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: var(--dp-agent);
  box-shadow: 0 0 0 2px rgba(239, 173, 5, 0.24);
}

.app-shell__tabs {
  display: flex;
  align-items: center;
  gap: 24px;
}
.app-shell__tab {
  color: rgba(255, 255, 255, 0.6);
  text-decoration: none;
  font: 500 13px var(--dp-font-body);
  padding: 4px 2px;
  border-radius: 0;
  transition: color 0.12s;
}
.app-shell__tab:hover { color: var(--dp-text-inverse); }
.app-shell__tab--active {
  color: var(--dp-text-inverse);
  font-weight: 700;
  box-shadow: inset 0 -2px 0 var(--dp-text-inverse);
}

.app-shell__spacer { flex: 1; }

.app-shell__right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.app-shell__icon-btn {
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.75);
  cursor: pointer;
  padding: 6px;
  display: inline-flex;
  align-items: center;
  border-radius: 0;
  text-decoration: none;
  transition: color 0.12s, background 0.12s;
}
.app-shell__icon-btn:hover { color: var(--dp-text-inverse); background: var(--dp-rail-soft); }
.app-shell__icon-btn--active { color: var(--dp-text-inverse); background: var(--dp-rail-soft); }
.app-shell__icon-btn:focus-visible {
  outline: 2px solid var(--dp-accent);
  outline-offset: 2px;
}

/* Agent Drawer toggle: gets its own class so it can host the red badge. */
.app-shell__agent-toggle {
  position: relative;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.75);
  cursor: pointer;
  padding: 4px 10px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 0;
  transition: color 0.12s, background 0.12s;
}
.app-shell__agent-toggle:hover { color: var(--dp-text-inverse); background: var(--dp-rail-soft); }
.app-shell__agent-toggle--active {
  background: var(--dp-agent);
  color: var(--dp-rail);
}
.app-shell__agent-toggle:focus-visible {
  outline: 2px solid var(--dp-accent);
  outline-offset: 2px;
}
.app-shell__agent-badge {
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  background: var(--dp-danger);
  color: var(--dp-text-inverse);
  border-radius: var(--dp-radius-pill);
  font: 700 10px var(--dp-font-data);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}

/* Avatar --------------------------------------------------------------- */
.app-shell__avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--dp-rail-soft);
  color: var(--dp-text-inverse);
  border: 1px solid rgba(255, 255, 255, 0.24);
  font: 700 11px var(--dp-font-body);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: background 0.12s, border-color 0.12s;
}
.app-shell__avatar:hover {
  background: #26476a;
  border-color: rgba(255, 255, 255, 0.4);
}
.app-shell__avatar:focus-visible {
  outline: 2px solid var(--dp-canvas);
  outline-offset: 2px;
}

/* Body ---------------------------------------------------------------- */
.app-shell__body {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: stretch;
}
.app-shell__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: var(--dp-canvas);
  overflow: hidden;
}

/* Status bar --------------------------------------------------------- */
.app-shell__statusbar {
  height: 28px;
  padding: 0 20px;
  background: var(--dp-rail-soft);
  color: var(--dp-ink-faint);
  font: 600 10px var(--dp-font-data);
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
  letter-spacing: 0.06em;
  border-radius: 0;
}
.app-shell__status-text { white-space: nowrap; }

@media (max-width: 820px) {
  .app-shell__tabs { display: none; }
  .app-shell__topbar { gap: 16px; padding: 0 12px; }
  .app-shell__agent-toggle { padding: 4px 6px; }
}
</style>
