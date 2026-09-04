<script setup lang="ts">
import { onMounted, ref } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import Skeleton from 'primevue/skeleton'
import { useToast } from 'primevue/usetoast'
import {
  agentDefinitionCatalogClient,
  type AgentDefinitionCatalogClient,
  type AgentDefinitionCatalogItem,
} from '@/api/agentDefinitions'
import Banner from '@/components/data/Banner.vue'
import AgentEmptyState from './AgentEmptyState.vue'
import { IconCpu, IconPlus, IconRefreshCw } from '@/components/icons'

const props = withDefaults(defineProps<{ client?: AgentDefinitionCatalogClient }>(), {
  client: () => agentDefinitionCatalogClient,
})

const toast = useToast()
type ViewState = 'loading' | 'ready' | 'unavailable'
const viewState = ref<ViewState>('loading')
const rows = ref<AgentDefinitionCatalogItem[]>([])
const nextCursor = ref('')
const bannerClosed = ref(false)
const createOpen = ref(false)
const creating = ref(false)

onMounted(() => { void load(true) })

async function load(replace: boolean) {
  if (replace) viewState.value = 'loading'
  try {
    const page = await props.client.list(replace ? '' : nextCursor.value, 50)
    rows.value = replace ? page.definitions : [...rows.value, ...page.definitions]
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    if (replace) rows.value = []
    viewState.value = 'unavailable'
  }
}

async function create() {
  if (!props.client.create) return
  creating.value = true
  try {
    const created = await props.client.create('subscription_autoreply')
    rows.value = [created, ...rows.value]
    createOpen.value = false
    toast.add({ severity: 'success', summary: 'Definition 已创建', detail: `${created.definitionId} v${created.version}`, life: 3000 })
  } catch {
    viewState.value = 'unavailable'
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <section class="drawer-view" data-agent-definitions-view>
    <header class="drawer-toolbar">
      <span class="drawer-title">定义</span>
      <span class="count-badge">{{ rows.length }}</span>
      <button type="button" class="icon-btn" title="刷新" @click="load(true)"><IconRefreshCw :size="14" /></button>
      <span class="spacer" />
      <button v-if="props.client.create" type="button" class="primary-btn" @click="createOpen = true">
        <IconPlus :size="14" /> 创建
      </button>
    </header>
    <Banner
      v-if="viewState === 'unavailable' && !bannerClosed"
      tone="danger"
      message="Definition 目录读取失败"
      action-label="Retry"
      @action="load(true)"
      @close="bannerClosed = true"
    />
    <div v-if="viewState === 'loading' && rows.length === 0" class="skel">
      <Skeleton v-for="n in 4" :key="n" height="32px" />
    </div>
    <AgentEmptyState
      v-else-if="viewState === 'ready' && rows.length === 0"
      :icon="IconCpu"
      title="还没有 Agent Definition"
      description="Definition 声明了一个 Agent 的行为版本 —— 它决定谁能创建订阅、能触发什么任务。至少发布一个 Definition 才能让 Agent 上岗。"
    >
      <button v-if="props.client.create" type="button" class="primary-btn" @click="createOpen = true">
        <IconPlus :size="14" /> 创建第一个
      </button>
    </AgentEmptyState>
    <DataTable v-else :value="rows" data-key="definitionId" size="small" striped-rows>
      <Column header="AGENT" field="agentId" />
      <Column header="ID">
        <template #body="{ data }">
          <span :data-agent-definition-id="data.definitionId">{{ data.definitionId }}</span>
        </template>
      </Column>
      <Column header="VER" field="version" style="width: 4rem" />
      <Column header="SCOPE">
        <template #body="{ data }">{{ data.conversationScopes.join(', ') }}</template>
      </Column>
    </DataTable>
    <button v-if="nextCursor" type="button" class="link" @click="load(false)">加载下一页</button>
    <Dialog v-model:visible="createOpen" modal header="创建 Definition" :style="{ width: 'min(420px, 92vw)' }">
      <p class="hint">将创建一个 subscription_autoreply profile 的 owner Definition。</p>
      <button type="button" class="primary-btn" :disabled="creating" @click="create">{{ creating ? '创建中…' : '确认创建' }}</button>
    </Dialog>
  </section>
</template>

<style scoped>
.drawer-view { display: flex; flex-direction: column; min-height: 100%; }
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
.icon-btn, .link { border: 0; background: transparent; color: var(--dp-accent); cursor: pointer; }
.icon-btn { color: var(--dp-ink-soft); padding: 6px; display: inline-flex; }
.icon-btn:hover { color: var(--dp-ink); background: var(--dp-surface-muted); }
.primary-btn { border: 0; background: var(--dp-accent); color: var(--dp-text-inverse); height: 30px; padding: 0 12px; font: 600 12px var(--dp-font-body); cursor: pointer; display: inline-flex; align-items: center; gap: 6px; }
.primary-btn:hover { background: var(--dp-accent-strong); }
.skel { display: grid; gap: 8px; padding: 12px; }
.hint { font: 400 13px var(--dp-font-body); color: var(--dp-ink-soft); margin-bottom: 12px; }
</style>
