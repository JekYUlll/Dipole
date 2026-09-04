<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import Skeleton from 'primevue/skeleton'
import { agentTaskClient, type AgentOwnedTask, type AgentTaskClient } from '@/api/agentTasks'
import { agentFlags, agentTaskCreatePageEnabled } from '@/config/agentFlags'
import Banner from '@/components/data/Banner.vue'
import StatusPill from '@/components/data/StatusPill.vue'
import AgentTaskCreate from '@/components/AgentTaskCreate.vue'
import AgentTaskTimeline from '@/components/AgentTaskTimeline.vue'
import AgentElicitationForm from '@/components/AgentElicitationForm.vue'
import AgentApprovalForm from '@/components/AgentApprovalForm.vue'
import AgentEmptyState from './AgentEmptyState.vue'
import { IconInbox, IconPlus, IconRefreshCw } from '@/components/icons'

const props = withDefaults(defineProps<{ client?: AgentTaskClient }>(), {
  client: () => agentTaskClient,
})

const route = useRoute()
const router = useRouter()
type ViewState = 'loading' | 'ready' | 'unavailable'
const viewState = ref<ViewState>('loading')
const tasks = ref<AgentOwnedTask[]>([])
const nextCursor = ref('')
const bannerClosed = ref(false)

const selectedId = computed(() => String(route.query.task ?? ''))
const panel = computed(() => {
  const raw = String(route.query.panel ?? 'timeline')
  return raw === 'input' || raw === 'approval' ? raw : 'timeline'
})
const createOpen = computed({
  get: () => route.query.drawer === 'create',
  set: (open: boolean) => toggleCreate(open),
})
const selected = computed(() => tasks.value.find(t => t.taskId === selectedId.value))

type Filter = 'all' | 'pending' | 'running'
const filter = ref<Filter>('all')
const pendingCount = computed(() => tasks.value.filter(t => t.pendingKind).length)
const runningCount = computed(() => tasks.value.filter(t => t.status === 'running').length)
const displayedTasks = computed(() => {
  if (filter.value === 'pending') return tasks.value.filter(t => t.pendingKind)
  if (filter.value === 'running') return tasks.value.filter(t => t.status === 'running')
  return tasks.value
})

function relative(ms: number) {
  const diff = Date.now() - ms
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  const days = Math.floor(diff / 86_400_000)
  if (days < 30) return `${days} 天前`
  return new Date(ms).toISOString().slice(0, 10)
}

onMounted(() => { void load(true) })

async function load(replace: boolean) {
  if (replace) viewState.value = 'loading'
  try {
    const list = props.client.list
    if (!list) throw new Error('unavailable')
    const page = await list(replace ? '' : nextCursor.value, 50)
    tasks.value = replace ? page.tasks : [...tasks.value, ...page.tasks]
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    if (replace) tasks.value = []
    viewState.value = 'unavailable'
  }
}

function openTask(item: AgentOwnedTask, nextPanel?: string) {
  const auto = item.pendingKind === 'approval' ? 'approval'
    : item.pendingKind === 'input' ? 'input'
    : 'timeline'
  router.replace({
    query: { ...route.query, agent: '1', view: 'tasks', task: item.taskId, panel: nextPanel ?? auto },
  })
}

function setPanel(next: string) {
  if (!selectedId.value) return
  router.replace({ query: { ...route.query, panel: next } })
}

function toggleCreate(open: boolean) {
  const query = { ...route.query, agent: '1', view: 'tasks' } as Record<string, string>
  if (open) query.drawer = 'create'
  else delete query.drawer
  router.replace({ query })
}

function onCreated(taskId: string) {
  router.replace({
    query: { agent: '1', view: 'tasks', task: taskId, panel: 'timeline' },
  })
  void load(true)
}

function closeDetail() {
  const query = { ...route.query }
  delete query.task
  delete query.panel
  router.replace({ query })
}

function tone(item: AgentOwnedTask): 'agent' | 'warning' | 'danger' | 'success' | 'neutral' {
  if (item.pendingKind) return 'warning'
  if (item.status === 'failed' || item.status === 'cancelled') return 'danger'
  if (item.status === 'completed') return 'success'
  if (item.status === 'running') return 'agent'
  return 'neutral'
}

function label(item: AgentOwnedTask) {
  if (item.pendingKind === 'approval') return 'AWAIT APPROVAL'
  if (item.pendingKind === 'input') return 'AWAIT INPUT'
  return item.status.replace(/_/g, ' ').toUpperCase()
}

function rowClass(row: AgentOwnedTask) {
  return row.taskId === selectedId.value ? 'is-selected' : ''
}
</script>

<template>
  <section class="drawer-view" data-agent-tasks-view :aria-busy="viewState === 'loading'">
    <header class="drawer-toolbar">
      <span class="drawer-title">任务</span>
      <span class="count-badge">{{ tasks.length }}</span>
      <button type="button" class="icon-btn" title="刷新" @click="load(true)"><IconRefreshCw :size="14" /></button>
      <span class="spacer" />
      <button
        v-if="agentTaskCreatePageEnabled"
        type="button"
        class="primary-btn"
        data-agent-task-create-entry
        @click="toggleCreate(true)"
      >
        <IconPlus :size="14" /> 创建任务
      </button>
    </header>

    <nav v-if="tasks.length > 0" class="filter-strip" aria-label="任务过滤">
      <button
        type="button"
        class="filter-chip"
        :class="{ 'filter-chip--active': filter === 'all' }"
        @click="filter = 'all'"
      >全部 <span class="filter-chip__count">{{ tasks.length }}</span></button>
      <button
        type="button"
        class="filter-chip filter-chip--pending"
        :class="{ 'filter-chip--active': filter === 'pending' }"
        @click="filter = 'pending'"
      >待我处理 <span class="filter-chip__count">{{ pendingCount }}</span></button>
      <button
        type="button"
        class="filter-chip filter-chip--running"
        :class="{ 'filter-chip--active': filter === 'running' }"
        @click="filter = 'running'"
      >运行中 <span class="filter-chip__count">{{ runningCount }}</span></button>
    </nav>

    <Banner
      v-if="viewState === 'unavailable' && !bannerClosed"
      tone="danger"
      message="任务列表读取失败"
      action-label="Retry"
      @action="load(true)"
      @close="bannerClosed = true"
    />

    <div v-if="viewState === 'loading' && tasks.length === 0" class="skel">
      <Skeleton v-for="n in 5" :key="n" height="32px" />
    </div>

    <div v-else class="split">
      <div class="list-pane">
        <AgentEmptyState
          v-if="viewState === 'ready' && tasks.length === 0"
          :icon="IconInbox"
          title="还没有任务"
          description="任务是 Agent 端可追溯的一次工作单：包含目标、时间线、审批与产物。创建任务后，你会在这里看到进度条、待办和状态流转。"
        >
          <button v-if="agentTaskCreatePageEnabled" type="button" class="primary-btn" @click="toggleCreate(true)">
            <IconPlus :size="14" /> 创建第一个任务
          </button>
        </AgentEmptyState>
        <DataTable
          v-else
          :value="displayedTasks"
          data-key="taskId"
          size="small"
          striped-rows
          :row-class="rowClass"
          @row-click="(e: { data: AgentOwnedTask }) => openTask(e.data)"
        >
          <Column header="STATUS" style="width: 7.5rem">
            <template #body="{ data }">
              <StatusPill :tone="tone(data)" :label="label(data)" dense />
            </template>
          </Column>
          <Column header="TASK" field="goal">
            <template #body="{ data }">
              <div class="task-cell">
                <strong>{{ data.goal || data.taskId }}</strong>
                <span class="task-cell__meta">
                  <span class="mono">{{ data.taskId }}</span>
                  <span v-if="data.updatedAtUnixMs" class="task-cell__time">· {{ relative(data.updatedAtUnixMs) }}</span>
                </span>
              </div>
            </template>
          </Column>
        </DataTable>
        <button v-if="nextCursor" type="button" class="link" :disabled="viewState === 'loading'" @click="load(false)">加载下一页</button>
      </div>

      <aside v-if="selectedId" class="detail-pane" :data-agent-task-panel="panel">
        <header class="detail-head">
          <span class="mono">{{ selectedId }}</span>
          <StatusPill v-if="selected" :tone="tone(selected)" :label="label(selected)" dense />
          <span class="spacer" />
          <button type="button" class="icon-btn" aria-label="关闭详情" @click="closeDetail">×</button>
        </header>
        <nav class="sub-tabs">
          <button type="button" :class="{ active: panel === 'timeline' }" @click="setPanel('timeline')">Timeline</button>
          <button
            v-if="agentFlags.elicitation"
            type="button"
            :class="{ active: panel === 'input' }"
            @click="setPanel('input')"
          >Input</button>
          <button
            v-if="agentFlags.approval"
            type="button"
            :class="{ active: panel === 'approval' }"
            @click="setPanel('approval')"
          >Approval</button>
        </nav>
        <AgentTaskTimeline v-if="panel === 'timeline'" :task-id="selectedId" :client="props.client" />
        <AgentElicitationForm v-else-if="panel === 'input'" embedded :task-id="selectedId" :client="props.client" />
        <AgentApprovalForm v-else-if="panel === 'approval'" embedded :task-id="selectedId" :client="props.client" />
      </aside>
    </div>

    <Dialog
      v-model:visible="createOpen"
      modal
      header="创建任务"
      :style="{ width: 'min(560px, 92vw)' }"
    >
      <AgentTaskCreate embedded :client="props.client" @created="onCreated" />
    </Dialog>
  </section>
</template>

<style scoped>
.drawer-view { display: flex; flex-direction: column; min-height: 100%; background: var(--dp-bg-workspace); }
.drawer-toolbar { display: flex; align-items: center; gap: 8px; height: 48px; padding: 0 14px; border-bottom: 1px solid var(--dp-line); background: var(--dp-surface); }
.drawer-title { font: 700 13px var(--dp-font-body); color: var(--dp-ink); }
.count-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 22px; height: 20px; padding: 0 7px;
  background: var(--dp-surface-muted); color: var(--dp-ink-soft);
  border: 1px solid var(--dp-line);
  font: 700 11px var(--dp-font-data); letter-spacing: 0.04em;
}
.spacer { flex: 1; }
.icon-btn, .link { border: 0; background: transparent; color: var(--dp-ink-soft); cursor: pointer; padding: 6px; display: inline-flex; align-items: center; }
.icon-btn:hover { color: var(--dp-ink); background: var(--dp-surface-muted); }
.link { color: var(--dp-accent); font: 700 12px var(--dp-font-body); padding: 8px 12px; }
.primary-btn { border: 0; background: var(--dp-accent); color: var(--dp-text-inverse); height: 30px; padding: 0 12px; font: 600 12px var(--dp-font-body); cursor: pointer; display: inline-flex; align-items: center; gap: 6px; }
.primary-btn:hover { background: var(--dp-accent-strong); }
.skel { display: grid; gap: 8px; padding: 12px; }
.split { display: flex; min-height: 0; flex: 1; }
.list-pane { flex: 1; min-width: 0; overflow: auto; }
.detail-pane { width: 52%; min-width: 220px; border-left: 1px solid var(--dp-line); background: var(--dp-surface); overflow: auto; display: flex; flex-direction: column; }
.detail-head { display: flex; align-items: center; gap: 8px; height: 40px; padding: 0 12px; border-bottom: 1px solid var(--dp-line); }
.sub-tabs { display: flex; gap: 4px; padding: 0 8px; height: 34px; border-bottom: 1px solid var(--dp-line); }
.sub-tabs button { border: 0; background: transparent; color: var(--dp-ink-faint); font: 500 11px var(--dp-font-body); cursor: pointer; border-bottom: 2px solid transparent; padding: 0 6px; }
.sub-tabs button.active { color: var(--dp-ink); font-weight: 700; border-bottom-color: var(--dp-accent); }
.task-cell { display: grid; gap: 2px; }
.task-cell strong { font: 600 12px var(--dp-font-body); color: var(--dp-ink); }
.task-cell__meta { display: inline-flex; gap: 6px; align-items: baseline; color: var(--dp-ink-faint); font-size: 10px; }
.task-cell__time { font: 500 10px var(--dp-font-body); }

.filter-strip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--dp-line);
  background: var(--dp-surface);
  flex-wrap: wrap;
}
.filter-chip {
  border: 1px solid var(--dp-line);
  background: transparent;
  color: var(--dp-ink-soft);
  padding: 4px 10px;
  cursor: pointer;
  font: 500 11px var(--dp-font-body);
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 999px;
  transition: color 120ms, background 120ms, border-color 120ms;
}
.filter-chip:hover { color: var(--dp-ink); border-color: var(--dp-ink-faint); }
.filter-chip--active {
  color: var(--dp-ink);
  background: var(--dp-surface-muted);
  border-color: var(--dp-ink-soft);
  font-weight: 700;
}
.filter-chip__count {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 18px; height: 16px; padding: 0 5px;
  background: var(--dp-surface-muted); color: var(--dp-ink-soft);
  border-radius: 999px;
  font: 700 10px var(--dp-font-data); letter-spacing: 0.04em;
}
.filter-chip--active .filter-chip__count { background: var(--dp-bg-panel); color: var(--dp-ink); }
.filter-chip--pending.filter-chip--active { color: var(--dp-warning); border-color: var(--dp-warning); background: var(--dp-warning-soft); }
.filter-chip--pending.filter-chip--active .filter-chip__count { background: var(--dp-warning); color: var(--dp-text-inverse); }
.filter-chip--running.filter-chip--active { color: var(--dp-agent-strong); border-color: var(--dp-agent); background: var(--dp-agent-soft); }
.filter-chip--running.filter-chip--active .filter-chip__count { background: var(--dp-agent); color: var(--dp-text-inverse); }
:deep(.is-selected) { background: var(--dp-surface-muted) !important; }
:deep(.p-datatable-tbody > tr) { cursor: pointer; }
</style>
