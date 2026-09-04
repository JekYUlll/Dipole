<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { agentFlags } from '@/config/agentFlags'
import { agentTaskClient, type AgentOwnedTask } from '@/api/agentTasks'
import { agentArtifactClient, type AgentArtifactMetadata } from '@/api/agentArtifacts'
import { useChatStore } from '@/stores/chat'
import StatusPill from '@/components/data/StatusPill.vue'
import Banner from '@/components/data/Banner.vue'
import { IconChevronRight, IconInbox, IconPackage, IconRefreshCw } from '@/components/icons'

const router = useRouter()
const route = useRoute()
const chat = useChatStore()
const loading = ref(false)
const tasks = ref<AgentOwnedTask[]>([])
const artifacts = ref<AgentArtifactMetadata[]>([])
const error = ref('')

const pending = computed(() => tasks.value.filter(t => t.pendingKind))
const current = computed(() => {
  const key = chat.activeKey
  const scoped = key ? tasks.value.filter(t => t.goal.includes(key) || t.taskId.includes(key)) : []
  return (scoped[0] ?? tasks.value.find(t => t.status === 'running') ?? tasks.value[0])
})
const recentArtifacts = computed(() => artifacts.value.slice(0, 3))

onMounted(() => { void load() })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [taskPage, artifactPage] = await Promise.all([
      agentFlags.timeline && agentTaskClient.list ? agentTaskClient.list('', 50) : Promise.resolve({ tasks: [], nextCursor: '' }),
      agentFlags.artifacts && agentArtifactClient.list ? agentArtifactClient.list('', 20) : Promise.resolve({ artifacts: [], nextCursor: '' }),
    ])
    tasks.value = taskPage.tasks
    artifacts.value = artifactPage.artifacts
  } catch {
    error.value = '现场数据读取失败'
  } finally {
    loading.value = false
  }
}

function go(view: string, extra: Record<string, string> = {}) {
  router.replace({ query: { ...route.query, agent: '1', view, ...extra } })
}

function openPending(item: AgentOwnedTask) {
  go('tasks', { task: item.taskId, panel: item.pendingKind === 'approval' ? 'approval' : 'input' })
}

function currentTone(): 'agent' | 'warning' | 'success' | 'danger' | 'neutral' {
  const c = current.value
  if (!c) return 'neutral'
  if (c.pendingKind) return 'warning'
  if (c.status === 'running') return 'agent'
  if (c.status === 'completed') return 'success'
  if (c.status === 'failed' || c.status === 'cancelled') return 'danger'
  return 'neutral'
}

function scopeLabel() {
  return chat.activeKey ? chat.activeKey : 'OWNER GLOBAL'
}
</script>

<template>
  <section class="live-view" aria-label="Agent 现场状态">
    <!-- Scope hero -------------------------------------------------- -->
    <header class="live-hero">
      <div class="live-hero__scope">
        <span class="live-hero__scope-chip">SCOPE</span>
        <span class="live-hero__scope-value mono">{{ scopeLabel() }}</span>
      </div>
      <div class="live-hero__body">
        <h2 class="live-hero__title">Agent 现场</h2>
        <p class="live-hero__caption">当前会话相关的任务、待处理与最近产物。没有会话时回退到账号全局 pending。</p>
      </div>
      <button
        type="button"
        class="live-hero__refresh"
        :disabled="loading"
        title="刷新"
        aria-label="刷新现场数据"
        @click="load"
      >
        <IconRefreshCw :size="14" />
      </button>
    </header>

    <Banner
      v-if="error"
      tone="danger"
      :message="error"
      action-label="刷新"
      :closable="false"
      @action="load"
    />

    <!-- KPI bar ------------------------------------------------------ -->
    <div class="live-kpis">
      <button
        type="button"
        class="live-kpi live-kpi--pending"
        :disabled="pending.length === 0"
        @click="pending.length && go('tasks')"
      >
        <span class="live-kpi__value">{{ pending.length }}</span>
        <span class="live-kpi__label">待处理</span>
      </button>
      <button
        type="button"
        class="live-kpi live-kpi--running"
        @click="go('tasks')"
      >
        <span class="live-kpi__value">{{ tasks.length }}</span>
        <span class="live-kpi__label">全部任务</span>
      </button>
      <button
        type="button"
        class="live-kpi live-kpi--artifacts"
        @click="go('artifacts')"
      >
        <span class="live-kpi__value">{{ artifacts.length }}</span>
        <span class="live-kpi__label">近期产物</span>
      </button>
    </div>

    <!-- Current task section ---------------------------------------- -->
    <section class="live-block" aria-labelledby="live-current-title">
      <header class="live-block__head">
        <h3 id="live-current-title" class="live-block__title">当前任务</h3>
        <span class="live-block__hint">最近一次进行中的任务</span>
      </header>
      <div v-if="current" class="live-current" data-live-current-task>
        <div class="live-current__row">
          <StatusPill :tone="currentTone()" :label="current.pendingKind ?? current.status" dense />
          <span class="mono live-current__id">{{ current.taskId }}</span>
        </div>
        <p class="live-current__goal">{{ current.goal || '（无目标摘要）' }}</p>
        <div class="live-current__actions">
          <button
            type="button"
            class="live-primary"
            @click="go('tasks', { task: current.taskId, panel: 'timeline' })"
          >
            打开时间线 <IconChevronRight :size="12" />
          </button>
          <button
            v-if="current.pendingKind"
            type="button"
            class="live-secondary"
            @click="openPending(current)"
          >
            {{ current.pendingKind === 'approval' ? '前往审批' : '前往输入' }}
          </button>
        </div>
      </div>
      <p v-else class="live-empty">当前没有进行中的任务</p>
    </section>

    <!-- Pending section --------------------------------------------- -->
    <section class="live-block" aria-labelledby="live-pending-title">
      <header class="live-block__head">
        <h3 id="live-pending-title" class="live-block__title">待我处理</h3>
        <span class="live-block__count">{{ pending.length }}</span>
      </header>
      <ul v-if="pending.length" class="live-list">
        <li v-for="item in pending.slice(0, 4)" :key="item.taskId">
          <button type="button" class="live-row live-row--pending" @click="openPending(item)">
            <StatusPill
              :tone="item.pendingKind === 'approval' ? 'danger' : 'warning'"
              :label="item.pendingKind ?? item.status"
              dense
            />
            <span class="live-row__body">
              <span class="live-row__label">{{ item.goal || item.taskId }}</span>
              <span class="mono live-row__meta">{{ item.taskId }}</span>
            </span>
            <span class="live-row__cta">
              {{ item.pendingKind === 'approval' ? '审批' : '处理' }}
              <IconChevronRight :size="12" />
            </span>
          </button>
        </li>
      </ul>
      <p v-else class="live-empty"><IconInbox :size="14" /> 没有等待你的输入或审批</p>
    </section>

    <!-- Recent artifacts section ------------------------------------ -->
    <section class="live-block" aria-labelledby="live-artifacts-title">
      <header class="live-block__head">
        <h3 id="live-artifacts-title" class="live-block__title">最近产物</h3>
        <span class="live-block__count">{{ recentArtifacts.length }}</span>
      </header>
      <ul v-if="recentArtifacts.length" class="live-list">
        <li v-for="item in recentArtifacts" :key="item.artifactId">
          <button type="button" class="live-row" @click="go('artifacts', { artifact: item.artifactId })">
            <span class="live-row__figure" aria-hidden="true"><IconPackage :size="14" /></span>
            <span class="live-row__body">
              <span class="live-row__label">{{ item.title }}</span>
              <span class="mono live-row__meta">{{ item.artifactType }}</span>
            </span>
            <IconChevronRight :size="12" class="live-row__chevron" />
          </button>
        </li>
      </ul>
      <p v-else class="live-empty"><IconPackage :size="14" /> 还没有产物</p>
    </section>
  </section>
</template>

<style scoped>
.live-view {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  min-height: 100%;
}

/* Hero -------------------------------------------------------------- */
.live-hero {
  display: grid;
  grid-template-columns: 1fr auto;
  grid-template-areas: 'scope refresh' 'body body';
  gap: 8px 8px;
  padding: 14px;
  background: var(--dp-surface);
  border: 1px solid var(--dp-line);
  border-top: 2px solid var(--dp-agent);
}
.live-hero__scope {
  grid-area: scope;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.live-hero__scope-chip {
  display: inline-flex; align-items: center;
  height: 18px; padding: 0 8px;
  background: transparent;
  color: var(--dp-ink-faint);
  font: 700 10px var(--dp-font-data);
  letter-spacing: 0.14em;
  border: 1px solid var(--dp-line);
}
.live-hero__scope-value {
  color: var(--dp-ink);
  font-size: 11px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.live-hero__body { grid-area: body; }
.live-hero__title {
  margin: 0 0 4px;
  font: 700 18px var(--dp-font-display);
  color: var(--dp-ink);
  line-height: 1.2;
}
.live-hero__caption {
  margin: 0;
  font: 400 12px var(--dp-font-body);
  color: var(--dp-ink-soft);
  line-height: 1.55;
}
.live-hero__refresh {
  grid-area: refresh;
  align-self: start;
  border: 1px solid var(--dp-line);
  background: var(--dp-bg-panel);
  color: var(--dp-ink-soft);
  width: 30px; height: 30px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.live-hero__refresh:hover:not(:disabled) { color: var(--dp-ink); background: var(--dp-surface-muted); }
.live-hero__refresh:disabled { opacity: 0.4; cursor: default; }

/* KPI bar ----------------------------------------------------------- */
.live-kpis {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.live-kpi {
  border: 1px solid var(--dp-line);
  background: var(--dp-surface);
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  cursor: pointer;
  text-align: left;
  transition: background 120ms, border-color 120ms, transform 120ms;
}
.live-kpi:hover:not(:disabled) { background: var(--dp-surface-muted); }
.live-kpi:disabled { cursor: default; opacity: 0.75; }
.live-kpi__value {
  font: 700 20px var(--dp-font-display);
  color: var(--dp-ink);
  line-height: 1;
}
.live-kpi__label {
  font: 500 11px var(--dp-font-body);
  color: var(--dp-ink-soft);
}
/* KPI cards share the same chrome; only the value color signals the
   category (pending → warning, running → agent-gold, artifacts → neutral).
   Uniform top border keeps the three cards visually grouped. */
.live-kpi { border-top: 2px solid var(--dp-line); }
.live-kpi--pending .live-kpi__value { color: var(--dp-warning); }
.live-kpi--running .live-kpi__value { color: var(--dp-agent-strong); }
.live-kpi--artifacts .live-kpi__value { color: var(--dp-ink); }

/* Section blocks --------------------------------------------------- */
.live-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.live-block__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 2px;
}
.live-block__title {
  margin: 0;
  font: 700 11px var(--dp-font-data);
  letter-spacing: 0.12em;
  color: var(--dp-ink-soft);
  text-transform: uppercase;
}
.live-block__count {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 18px; padding: 0 6px;
  background: var(--dp-surface-muted); color: var(--dp-ink-soft);
  border: 1px solid var(--dp-line);
  font: 700 10px var(--dp-font-data); letter-spacing: 0.04em;
}
.live-block__hint {
  font: 400 11px var(--dp-font-body);
  color: var(--dp-ink-faint);
  margin-left: auto;
}

/* Current task ------------------------------------------------------ */
.live-current {
  background: var(--dp-surface);
  border: 1px solid var(--dp-line);
  padding: 12px 14px;
  display: grid;
  gap: 10px;
}
.live-current__row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.live-current__id { color: var(--dp-ink-soft); font-size: 10px; }
.live-current__goal {
  margin: 0;
  font: 600 13px var(--dp-font-body);
  color: var(--dp-ink);
  line-height: 1.4;
}
.live-current__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.live-primary,
.live-secondary {
  border: 0;
  cursor: pointer;
  font: 600 12px var(--dp-font-body);
  height: 30px;
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.live-primary { background: var(--dp-accent); color: var(--dp-text-inverse); }
.live-primary:hover { background: var(--dp-accent-strong); }
.live-secondary {
  background: transparent;
  color: var(--dp-ink);
  border: 1px solid var(--dp-line);
}
.live-secondary:hover { background: var(--dp-surface-muted); }

/* Row lists --------------------------------------------------------- */
.live-list { list-style: none; margin: 0; padding: 0; display: grid; gap: 6px; }
.live-row {
  width: 100%;
  border: 1px solid var(--dp-line);
  background: var(--dp-surface);
  padding: 10px 12px;
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  text-align: left;
  transition: background 120ms, border-color 120ms;
}
.live-row:hover { background: var(--dp-surface-muted); border-color: var(--dp-ink-faint); }
.live-row--pending { border-left: 2px solid var(--dp-warning); }
.live-row__figure {
  width: 26px; height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--dp-surface-muted);
  border: 1px solid var(--dp-line);
  color: var(--dp-ink-soft);
  flex-shrink: 0;
}
.live-row__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.live-row__label {
  font: 600 12px var(--dp-font-body);
  color: var(--dp-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.live-row__meta {
  color: var(--dp-ink-faint);
  font-size: 10px;
}
.live-row__cta {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font: 700 11px var(--dp-font-body);
  color: var(--dp-accent);
  flex-shrink: 0;
}
.live-row__chevron { color: var(--dp-ink-faint); flex-shrink: 0; }

/* Empty inline ------------------------------------------------------ */
.live-empty {
  margin: 0;
  padding: 12px 14px;
  border: 1px dashed var(--dp-line);
  background: transparent;
  color: var(--dp-ink-faint);
  font: 500 12px var(--dp-font-body);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
