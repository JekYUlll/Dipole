<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Skeleton from 'primevue/skeleton'
import { agentArtifactClient, type AgentArtifactClient, type AgentArtifactMetadata } from '@/api/agentArtifacts'
import Banner from '@/components/data/Banner.vue'
import AgentArtifactMetadataPanel from '@/components/AgentArtifactMetadata.vue'
import AgentEmptyState from './AgentEmptyState.vue'
import { IconPackage, IconRefreshCw } from '@/components/icons'

const props = withDefaults(defineProps<{ client?: AgentArtifactClient }>(), {
  client: () => agentArtifactClient,
})

const route = useRoute()
const router = useRouter()
type ViewState = 'loading' | 'ready' | 'unavailable'
const viewState = ref<ViewState>('loading')
const rows = ref<AgentArtifactMetadata[]>([])
const nextCursor = ref('')
const bannerClosed = ref(false)
const selectedId = computed(() => String(route.query.artifact ?? ''))

onMounted(() => { void load(true) })

async function load(replace: boolean) {
  if (replace) viewState.value = 'loading'
  try {
    const list = props.client.list
    if (!list) throw new Error('unavailable')
    const page = await list(replace ? '' : nextCursor.value, 50)
    rows.value = replace ? page.artifacts : [...rows.value, ...page.artifacts]
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    if (replace) rows.value = []
    viewState.value = 'unavailable'
  }
}

function open(item: AgentArtifactMetadata) {
  router.replace({ query: { ...route.query, agent: '1', view: 'artifacts', artifact: item.artifactId } })
}

function closeDetail() {
  const query = { ...route.query }
  delete query.artifact
  router.replace({ query })
}

function sizeLabel(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function onArtifactRowClick(event: { data: AgentArtifactMetadata }) {
  open(event.data)
}
</script>

<template>
  <section class="drawer-view" data-agent-artifacts-view>
    <header class="drawer-toolbar">
      <span class="drawer-title">产物</span>
      <span class="count-badge">{{ rows.length }}</span>
      <button type="button" class="icon-btn" title="刷新" @click="load(true)"><IconRefreshCw :size="14" /></button>
    </header>
    <Banner
      v-if="viewState === 'unavailable' && !bannerClosed"
      tone="danger"
      message="产物列表读取失败"
      action-label="Retry"
      @action="load(true)"
      @close="bannerClosed = true"
    />
    <div v-if="viewState === 'loading' && rows.length === 0" class="skel">
      <Skeleton v-for="n in 4" :key="n" height="32px" />
    </div>
    <div v-else class="split">
      <div class="list-pane">
        <AgentEmptyState
          v-if="viewState === 'ready' && rows.length === 0"
          :icon="IconPackage"
          title="还没有产物"
          description="Agent 任务完成后落地的可下载资产（会话摘要、报表、生成的文件）会归档在这里。跑一个任务或触发一次订阅，第一份产物就会出现。"
        />
        <DataTable
          v-else
          :value="rows"
          data-key="artifactId"
          size="small"
          striped-rows
          @row-click="onArtifactRowClick"
        >
          <Column header="TITLE" field="title" />
          <Column header="TYPE" field="artifactType" />
          <Column header="SIZE">
            <template #body="{ data }">{{ sizeLabel(data.sizeBytes) }}</template>
          </Column>
        </DataTable>
        <button v-if="nextCursor" type="button" class="link" @click="load(false)">加载下一页</button>
      </div>
      <aside v-if="selectedId" class="detail-pane">
        <header class="detail-head">
          <span class="mono">{{ selectedId.slice(0, 12) }}…</span>
          <span class="spacer" />
          <button type="button" class="icon-btn" aria-label="关闭详情" @click="closeDetail">×</button>
        </header>
        <AgentArtifactMetadataPanel embedded :artifact-id="selectedId" :client="props.client" />
      </aside>
    </div>
  </section>
</template>

<style scoped>
.drawer-view { display: flex; flex-direction: column; min-height: 100%; }
.drawer-toolbar { display: flex; align-items: center; gap: 8px; height: 48px; padding: 0 14px; border-bottom: 1px solid var(--dp-line); background: var(--dp-surface); }
.detail-head { display: flex; align-items: center; gap: 8px; height: 40px; padding: 0 12px; border-bottom: 1px solid var(--dp-line); background: var(--dp-surface); }
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
.skel { display: grid; gap: 8px; padding: 12px; }
.split { display: flex; flex: 1; min-height: 0; }
.list-pane { flex: 1; min-width: 0; overflow: auto; }
.detail-pane { width: 52%; border-left: 1px solid var(--dp-line); overflow: auto; background: var(--dp-surface); }
:deep(.p-datatable-tbody > tr) { cursor: pointer; }
</style>
