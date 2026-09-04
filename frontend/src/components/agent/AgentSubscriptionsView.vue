<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import ConfirmDialog from 'primevue/confirmdialog'
import Select from 'primevue/select'
import RadioButton from 'primevue/radiobutton'
import InputText from 'primevue/inputtext'
import Skeleton from 'primevue/skeleton'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import {
  agentSubscriptionClient,
  type AgentSubscription,
  type AgentSubscriptionClient,
  type AgentSubscriptionConversationOption,
  type AgentSubscriptionFilterKind,
} from '@/api/agentSubscriptions'
import {
  agentDefinitionCatalogClient,
  type AgentDefinitionCatalogClient,
  type AgentDefinitionCatalogItem,
} from '@/api/agentDefinitions'
import Banner from '@/components/data/Banner.vue'
import StatusPill from '@/components/data/StatusPill.vue'
import AgentEmptyState from './AgentEmptyState.vue'
import { IconPlus, IconRadio, IconRefreshCw } from '@/components/icons'

const props = withDefaults(defineProps<{
  client?: AgentSubscriptionClient
  definitionClient?: AgentDefinitionCatalogClient
}>(), {
  client: () => agentSubscriptionClient,
  definitionClient: () => agentDefinitionCatalogClient,
})

const confirm = useConfirm()
const toast = useToast()
type ViewState = 'loading' | 'ready' | 'unavailable' | 'stale'
const viewState = ref<ViewState>('loading')
const rows = ref<AgentSubscription[]>([])
const nextCursor = ref('')
const bannerClosed = ref(false)
const createOpen = ref(false)
const createBusy = ref(false)
const definitions = ref<AgentDefinitionCatalogItem[]>([])
const selectedDefinitionIndex = ref(0)
const conversations = ref<AgentSubscriptionConversationOption[]>([])
const selectedConversation = ref('')
const filterKind = ref<AgentSubscriptionFilterKind>('all')
const termsText = ref('')
const createError = ref('')

const selectedDefinition = computed(() => definitions.value[selectedDefinitionIndex.value])
const canSubmit = computed(() => Boolean(selectedDefinition.value && selectedConversation.value) && !createBusy.value
  && (filterKind.value === 'all' || parseTerms().length > 0))

onMounted(() => { void load(true) })

async function load(replace: boolean) {
  if (replace) viewState.value = 'loading'
  try {
    const page = await props.client.list(replace ? '' : nextCursor.value, 50)
    rows.value = replace ? page.subscriptions : [...rows.value, ...page.subscriptions]
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    if (replace) rows.value = []
    viewState.value = 'unavailable'
  }
}

async function openCreate() {
  createOpen.value = true
  createError.value = ''
  createBusy.value = true
  try {
    const page = await props.definitionClient.list('', 100)
    definitions.value = page.definitions
    selectedDefinitionIndex.value = 0
    if (!definitions.value.length) {
      createError.value = '没有可用的 active Definition'
      return
    }
    await loadConversations()
  } catch {
    createError.value = '读取 Definition 失败'
  } finally {
    createBusy.value = false
  }
}

async function loadConversations() {
  const def = selectedDefinition.value
  conversations.value = []
  selectedConversation.value = ''
  if (!def) return
  createBusy.value = true
  try {
    const options = await props.client.listEligibleConversations(def.definitionId, def.version)
    conversations.value = options.conversations
    selectedConversation.value = options.conversations[0]?.conversationKey ?? ''
  } catch {
    createError.value = 'Definition 已失效，请重新读取'
    viewState.value = 'stale'
  } finally {
    createBusy.value = false
  }
}

function parseTerms() {
  return [...new Set(termsText.value.split(/[，,\n]/u).map(t => t.trim()).filter(Boolean))]
}

async function submitCreate() {
  const def = selectedDefinition.value
  if (!def || !selectedConversation.value) return
  createBusy.value = true
  createError.value = ''
  try {
    const created = await props.client.create({
      definitionId: def.definitionId,
      definitionVersion: def.version,
      conversationKey: selectedConversation.value,
      filterKind: filterKind.value,
      filter: filterKind.value === 'all' ? {} : { terms: parseTerms() },
    })
    rows.value = [created, ...rows.value.filter(r => r.subscriptionId !== created.subscriptionId)]
    createOpen.value = false
    toast.add({ severity: 'success', summary: '订阅已创建', detail: `${created.agentId} → ${created.resourceId}`, life: 3000 })
  } catch (error) {
    createError.value = error instanceof Error && /denied|scope/i.test(error.message) ? '会话权限或 scope 已变化' : '创建失败'
  } finally {
    createBusy.value = false
  }
}

function askRevoke(item: AgentSubscription) {
  confirm.require({
    header: '撤销订阅',
    message: `撤销 ${item.agentId} 绑定 ${item.resourceId}？该动作会留下审计记录。`,
    acceptLabel: '确认撤销',
    rejectLabel: '取消',
    acceptClass: 'p-button-danger',
    accept: async () => {
      try {
        const revoked = await props.client.revoke(item.subscriptionId, 'owner revoked')
        rows.value = rows.value.map(r => r.subscriptionId === revoked.subscriptionId ? revoked : r)
        toast.add({ severity: 'success', summary: '已撤销', life: 2500 })
      } catch (error) {
        viewState.value = error instanceof Error && /conflict|stale|changed/i.test(error.message) ? 'stale' : 'unavailable'
      }
    },
  })
}

function filterLabel(item: AgentSubscription) {
  return item.filterKind === 'all' ? 'all messages' : `contains · ${(item.filter.terms ?? []).join(' / ')}`
}
</script>

<template>
  <section class="drawer-view" data-agent-subscriptions-view :aria-busy="viewState === 'loading'">
    <ConfirmDialog />
    <header class="drawer-toolbar">
      <span class="drawer-title">订阅</span>
      <span class="count-badge">{{ rows.length }}</span>
      <button type="button" class="icon-btn" title="刷新" @click="load(true)"><IconRefreshCw :size="14" /></button>
      <span class="spacer" />
      <button type="button" class="primary-btn" data-agent-subscription-create :disabled="viewState === 'loading'" @click="openCreate">
        <IconPlus :size="14" /> 创建订阅
      </button>
    </header>

    <Banner
      v-if="viewState === 'unavailable' && !bannerClosed"
      tone="danger"
      message="订阅列表读取失败 · 授权或网络异常"
      action-label="Retry"
      @action="load(true)"
      @close="bannerClosed = true"
    />
    <Banner
      v-else-if="viewState === 'stale' && !bannerClosed"
      tone="warning"
      message="Definition 已失效，撤销或创建未被接受"
      action-label="重新读取"
      @action="load(true)"
      @close="bannerClosed = true"
    />

    <div v-if="viewState === 'loading' && rows.length === 0" class="skel">
      <Skeleton v-for="n in 4" :key="n" height="32px" />
    </div>

    <AgentEmptyState
      v-else-if="viewState === 'ready' && rows.length === 0"
      :icon="IconRadio"
      title="还没有事件订阅"
      description="订阅让 Agent 监听指定会话里符合关键词的消息，触发后台任务或摘要。创建订阅前需要至少一个活跃的 Definition。"
    >
      <button type="button" class="primary-btn" data-agent-subscription-create-empty @click="openCreate">
        <IconPlus :size="14" /> 创建第一个订阅
      </button>
    </AgentEmptyState>

    <DataTable v-else :value="rows" data-key="subscriptionId" size="small" striped-rows>
      <Column header="STATUS" style="width: 6.5rem">
        <template #body="{ data }">
          <StatusPill :tone="data.status === 'active' ? 'agent' : 'danger'" :label="data.status" dense />
        </template>
      </Column>
      <Column header="AGENT">
        <template #body="{ data }">
          <span :data-agent-subscription-id="data.subscriptionId">{{ data.agentId }}</span>
        </template>
      </Column>
      <Column header="SCOPE">
        <template #body="{ data }"><span class="mono">{{ data.resourceId }}</span></template>
      </Column>
      <Column header="FILTER">
        <template #body="{ data }">{{ filterLabel(data) }}</template>
      </Column>
      <Column header="" style="width: 4.5rem">
        <template #body="{ data }">
          <button
            v-if="data.status === 'active'"
            type="button"
            class="link"
            :data-agent-subscription-revoke="data.subscriptionId"
            @click="askRevoke(data)"
          >撤销</button>
        </template>
      </Column>
    </DataTable>
    <button v-if="nextCursor" type="button" class="link" @click="load(false)">加载下一页</button>

    <Dialog v-model:visible="createOpen" modal header="创建事件订阅" :style="{ width: 'min(520px, 92vw)' }">
      <p v-if="createBusy" class="hint">正在读取候选…</p>
      <div v-else class="create-form">
        <label>Active Definition
          <Select
            v-model="selectedDefinitionIndex"
            :options="definitions.map((d, i) => ({ label: `${d.agentId} · ${d.definitionId} v${d.version}`, value: i }))"
            option-label="label"
            option-value="value"
            data-agent-subscription-definition
            @change="loadConversations"
          />
        </label>
        <label>Readable ∩ Scope
          <Select
            v-model="selectedConversation"
            :options="conversations"
            option-label="conversationKey"
            option-value="conversationKey"
            data-agent-subscription-conversation
          />
        </label>
        <fieldset>
          <legend>Filter</legend>
          <label class="radio"><RadioButton v-model="filterKind" value="all" /> 全部消息</label>
          <label class="radio"><RadioButton v-model="filterKind" value="message_contains_any" /> 包含关键词</label>
          <InputText v-if="filterKind === 'message_contains_any'" v-model="termsText" data-agent-subscription-terms placeholder="事故, 延期" />
        </fieldset>
        <p v-if="createError" class="error" role="alert">{{ createError }}</p>
        <button type="button" class="primary-btn" data-agent-subscription-submit :disabled="!canSubmit" @click="submitCreate">
          {{ createBusy ? '正在创建…' : '创建控制记录' }}
        </button>
      </div>
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
.icon-btn, .link { border: 0; background: transparent; color: var(--dp-accent); cursor: pointer; font: 700 12px var(--dp-font-body); }
.icon-btn { color: var(--dp-ink-soft); padding: 6px; display: inline-flex; }
.icon-btn:hover { color: var(--dp-ink); background: var(--dp-surface-muted); }
.primary-btn { border: 0; background: var(--dp-accent); color: var(--dp-text-inverse); height: 30px; padding: 0 12px; font: 600 12px var(--dp-font-body); cursor: pointer; display: inline-flex; align-items: center; gap: 6px; }
.primary-btn:hover:not(:disabled) { background: var(--dp-accent-strong); }
.primary-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.skel { display: grid; gap: 8px; padding: 12px; }
.create-form { display: grid; gap: 12px; }
.create-form label { display: grid; gap: 6px; font: 600 12px var(--dp-font-body); }
.radio { display: flex; align-items: center; gap: 8px; font-weight: 500; }
.hint, .error { font: 500 12px var(--dp-font-body); }
.error { color: var(--dp-danger); }
fieldset { border: 1px solid var(--dp-line); padding: 10px; display: grid; gap: 8px; }
legend { font: 700 10px var(--dp-font-data); letter-spacing: 0.08em; color: var(--dp-ink-soft); }
</style>
