<script setup lang="ts">
// AgentDrawer — the single canonical entry point for every Agent surface.
//
// Layout: 420px right-side inline panel that shares the AppShell body with
// the main Chat pane; it does NOT overlay content. Six tabs (Live, Tasks,
// Artifacts, Definitions, Subscriptions, Memories) map 1:1 to `view` URL
// query values. See docs/notes/frontend-bi-redesign.md §3 (Chat + Agent
// Drawer IA) and §3.5 (Kill List — the 10 legacy /agent/* routes that
// this Drawer replaces).
//
// State model:
//   - Visibility: `?agent=1` from AppShell top-bar toggle.
//   - Active view: `?view=<key>` (default: live).
//   - Feature-flagged per view via config/agentFlags. Views the owner
//     cannot see are simply not rendered in the tab bar; the Drawer never
//     shows an empty tab.
//   - Close: × icon, Escape key, or clicking the Drawer toggle again.
//
// The Drawer never scrolls — its body owns the scroll surface so the tab
// bar and footer stay pinned during long timelines / candidate lists.

import { computed, defineAsyncComponent, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Toast from 'primevue/toast'
import { agentFlags } from '@/config/agentFlags'
import {
  IconClose, IconRadio, IconInbox, IconPackage, IconGrid, IconRefreshCw, IconCpu,
} from '@/components/icons'

// Lazy-load each view so the Drawer only pays for what the owner opens.
const AgentLiveView          = defineAsyncComponent(() => import('./AgentLiveView.vue'))
const AgentTasksView         = defineAsyncComponent(() => import('./AgentTasksView.vue'))
const AgentArtifactsView     = defineAsyncComponent(() => import('./AgentArtifactsView.vue'))
const AgentDefinitionsView   = defineAsyncComponent(() => import('./AgentDefinitionsView.vue'))
const AgentSubscriptionsView = defineAsyncComponent(() => import('./AgentSubscriptionsView.vue'))
const AgentMemoriesView      = defineAsyncComponent(() => import('./AgentMemoriesView.vue'))

export type AgentDrawerView =
  | 'live' | 'tasks' | 'artifacts' | 'definitions' | 'subscriptions' | 'memories'

interface TabDef {
  key: AgentDrawerView
  label: string
  icon: typeof IconCpu
  visible: boolean
}

const props = withDefaults(defineProps<{
  /** External override — when true, `?agent=1` is ignored. Used for tests. */
  forceOpen?: boolean
}>(), { forceOpen: false })

const router = useRouter()
const route = useRoute()

const isOpen = computed(() => props.forceOpen || route.query.agent === '1')

// Fixed tab order — regardless of route or feature flag, tabs never reshuffle.
const tabs = computed<TabDef[]>(() => [
  { key: 'live',          label: '现场',   icon: IconRefreshCw, visible: true },
  { key: 'tasks',         label: '任务',   icon: IconInbox,     visible: agentFlags.taskCreate || agentFlags.timeline },
  { key: 'artifacts',     label: '产物',   icon: IconPackage,   visible: agentFlags.artifacts },
  { key: 'definitions',   label: '定义',   icon: IconCpu,       visible: agentFlags.definitions },
  { key: 'subscriptions', label: '订阅',   icon: IconRadio,     visible: agentFlags.subscriptions },
  { key: 'memories',      label: '记忆',   icon: IconGrid,      visible: agentFlags.memories },
])

const visibleTabs = computed(() => tabs.value.filter(t => t.visible))

const activeView = computed<AgentDrawerView>(() => {
  const wanted = String(route.query.view ?? '')
  const found = visibleTabs.value.find(t => t.key === wanted)
  // If the URL asks for a hidden or unknown view we fall back to Live so the
  // Drawer never renders a broken empty tab pane.
  return (found?.key ?? 'live') as AgentDrawerView
})

function selectView(key: AgentDrawerView) {
  if (key === activeView.value) return
  // Preserve any other agent-related query (e.g. `task`, `artifact`); replace
  // instead of push so tab flicking doesn't pollute the back stack.
  router.replace({ query: { ...route.query, agent: '1', view: key } })
}

function close() {
  const q = { ...route.query }
  delete q.agent
  delete q.view
  delete q.task
  delete q.artifact
  delete q.panel
  delete q.drawer
  router.replace({ query: q })
}

// Escape closes the drawer. We bind at window scope so the drawer can trap
// closing regardless of which pane the user last focused.
function onKeydown(e: KeyboardEvent) {
  if (!isOpen.value) return
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

// If the drawer opens without a `view` query we normalise it to `live` so
// deep-links share the same key format as the internal state.
watch(isOpen, opened => {
  if (opened && !route.query.view) {
    router.replace({ query: { ...route.query, view: 'live' } })
  }
})
</script>

<template>
  <aside
    v-if="isOpen"
    class="agent-drawer"
    role="complementary"
    aria-label="Agent 抽屉"
    :data-agent-drawer-view="activeView"
  >
    <Toast position="bottom-right" />
    <!-- Header 56px: brand title + control-plane eyebrow + close × -->
    <header class="agent-drawer__header">
      <div class="agent-drawer__title">
        <span class="agent-drawer__title-dot" aria-hidden="true" />
        <div class="agent-drawer__title-text">
          <span class="agent-drawer__title-name">AGENT</span>
          <span class="agent-drawer__title-sub">Control plane · {{ visibleTabs.length }} views</span>
        </div>
      </div>
      <button
        type="button"
        class="agent-drawer__close"
        aria-label="关闭 Agent 抽屉"
        data-agent-drawer-close
        @click="close"
      >
        <IconClose :size="16" />
      </button>
    </header>

    <!-- Tab bar 44px -->
    <nav class="agent-drawer__tabs" role="tablist">
      <button
        v-for="t in visibleTabs" :key="t.key"
        type="button"
        role="tab"
        class="agent-drawer__tab"
        :class="{ 'agent-drawer__tab--active': activeView === t.key }"
        :aria-selected="activeView === t.key"
        :data-agent-drawer-tab="t.key"
        @click="selectView(t.key)"
      >
        <component :is="t.icon" :size="14" />
        <span>{{ t.label }}</span>
      </button>
    </nav>

    <!-- Body owns the scroll surface -->
    <div class="agent-drawer__body">
      <AgentLiveView          v-if="activeView === 'live'" />
      <AgentTasksView         v-else-if="activeView === 'tasks'" />
      <AgentArtifactsView     v-else-if="activeView === 'artifacts'" />
      <AgentDefinitionsView   v-else-if="activeView === 'definitions'" />
      <AgentSubscriptionsView v-else-if="activeView === 'subscriptions'" />
      <AgentMemoriesView      v-else-if="activeView === 'memories'" />
    </div>
  </aside>
</template>

<style scoped>
.agent-drawer {
  width: 420px;
  min-width: 420px;
  max-width: 40vw;
  height: 100%;
  background: var(--dp-bg-panel);
  border-left: 1px solid var(--dp-line);
  display: flex;
  flex-direction: column;
  border-radius: 0;
  flex-shrink: 0;
}

/* Header -------------------------------------------------------------- */
.agent-drawer__header {
  height: 56px;
  padding: 0 16px;
  border-bottom: 1px solid var(--dp-line);
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
  background: var(--dp-bg-panel);
  position: relative;
}
.agent-drawer__header::after {
  /* single-pixel accent — no visual competition with tab underline */
  content: '';
  position: absolute;
  left: 0; right: 0; bottom: -1px;
  height: 1px;
  background: var(--dp-line);
  pointer-events: none;
}
.agent-drawer__title {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--dp-ink);
  flex: 1;
  min-width: 0;
}
.agent-drawer__title-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--dp-agent);
  flex-shrink: 0;
}
.agent-drawer__title-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.agent-drawer__title-name {
  font: 800 13px var(--dp-font-data);
  letter-spacing: 0.16em;
  color: var(--dp-ink);
  line-height: 1;
}
.agent-drawer__title-sub {
  font: 500 10px var(--dp-font-body);
  color: var(--dp-ink-faint);
  letter-spacing: 0.04em;
  line-height: 1;
}
.agent-drawer__close {
  border: 1px solid transparent;
  background: transparent;
  color: var(--dp-ink-soft);
  cursor: pointer;
  width: 32px; height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0;
  transition: background 120ms, color 120ms, border-color 120ms;
}
.agent-drawer__close:hover { color: var(--dp-ink); background: var(--dp-surface-muted); border-color: var(--dp-line); }
.agent-drawer__close:focus-visible {
  outline: 2px solid var(--dp-accent);
  outline-offset: 2px;
}

/* Tab bar ------------------------------------------------------------ */
.agent-drawer__tabs {
  height: 44px;
  padding: 0 6px;
  border-bottom: 1px solid var(--dp-line);
  display: flex;
  align-items: stretch;
  gap: 0;
  background: var(--dp-bg-panel);
  overflow-x: auto;
  flex-shrink: 0;
}
.agent-drawer__tab {
  border: 0;
  background: transparent;
  color: var(--dp-ink-soft);
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font: 500 12px var(--dp-font-body);
  cursor: pointer;
  border-radius: 0;
  border-bottom: 2px solid transparent;
  white-space: nowrap;
  flex-shrink: 0;
  flex: 1;
  min-width: 0;
  transition: color 120ms, background 120ms, border-color 120ms;
  position: relative;
}
.agent-drawer__tab :first-child { opacity: 0.7; transition: opacity 120ms; }
.agent-drawer__tab:hover { color: var(--dp-ink); background: var(--dp-surface-muted); }
.agent-drawer__tab:hover :first-child { opacity: 1; }
.agent-drawer__tab--active {
  color: var(--dp-ink);
  font-weight: 700;
  border-bottom-color: var(--dp-agent);
}
.agent-drawer__tab--active :first-child { opacity: 1; color: var(--dp-agent-strong); }
.agent-drawer__tab:focus-visible {
  outline: 2px solid var(--dp-agent);
  outline-offset: -2px;
}

/* Body owns the scroll surface. Each view is responsible for its own
   internal layout / toolbar. */
.agent-drawer__body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  background: var(--dp-bg-workspace);
}

@media (max-width: 1180px) {
  /* Below ~1180px the side-by-side layout squeezes the Chat pane to under
     280px, so the drawer switches to a right-anchored overlay covering the
     session list and main pane. The left nav rail stays visible so users
     can still click away. Chat continues to render behind. */
  .agent-drawer {
    position: fixed;
    top: 48px; bottom: 28px; right: 0;
    width: min(460px, calc(100vw - 48px));
    min-width: 0;
    max-width: none;
    z-index: 40;
    box-shadow: -12px 0 32px rgba(9, 37, 69, 0.14);
  }
}
@media (max-width: 640px) {
  .agent-drawer {
    width: calc(100vw - 48px);
  }
}
</style>
