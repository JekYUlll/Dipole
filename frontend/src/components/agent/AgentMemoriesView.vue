<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Skeleton from 'primevue/skeleton'
import { useToast } from 'primevue/usetoast'
import {
  agentMemoryClient,
  type AgentMemory,
  type AgentMemoryCandidate,
  type AgentMemoryClient,
} from '@/api/agentMemories'
import { agentFlags } from '@/config/agentFlags'
import Banner from '@/components/data/Banner.vue'
import StatusPill from '@/components/data/StatusPill.vue'
import { IconRefreshCw } from '@/components/icons'

const props = withDefaults(defineProps<{ client?: AgentMemoryClient }>(), {
  client: () => agentMemoryClient,
})

const toast = useToast()
type ViewState = 'loading' | 'ready' | 'unavailable' | 'conflict'
const viewState = ref<ViewState>('loading')
const memories = ref<AgentMemory[]>([])
const candidates = ref<AgentMemoryCandidate[]>([])
const bannerClosed = ref(false)
const correcting = ref<AgentMemory>()
const content = ref('')
const compact = ref('')
const reason = ref('')
const correctOpen = computed({
  get: () => correcting.value !== undefined,
  set: (open: boolean) => { if (!open) correcting.value = undefined },
})

onMounted(() => { void load() })

async function load() {
  viewState.value = 'loading'
  try {
    const [mem, cand] = await Promise.all([
      props.client.list('', 50),
      props.client.listCandidates('', 50),
    ])
    memories.value = mem.memories
    candidates.value = cand.candidates
    viewState.value = 'ready'
  } catch {
    viewState.value = 'unavailable'
  }
}

async function review(item: AgentMemoryCandidate, decision: 'accepted' | 'rejected') {
  try {
    const updated = await props.client.reviewCandidate(item.candidateId, item.candidateSha256, decision, `owner ${decision}`)
    candidates.value = candidates.value.map(c => c.candidateId === updated.candidateId ? updated : c)
  } catch {
    viewState.value = 'unavailable'
  }
}

async function promote(item: AgentMemoryCandidate) {
  if (!item.reviewId) return
  try {
    const memory = await props.client.promoteCandidate(item.candidateId, item.candidateSha256, item.reviewId)
    memories.value = [memory, ...memories.value]
    toast.add({ severity: 'success', summary: '已晋升为长期记忆', life: 2500 })
  } catch {
    viewState.value = 'conflict'
  }
}

function startCorrect(item: AgentMemory) {
  correcting.value = item
  content.value = item.content
  compact.value = item.compactContent ?? ''
  reason.value = ''
}

async function submitCorrect() {
  const item = correcting.value
  if (!item) return
  try {
    await props.client.correct(item.memoryId, item.memoryVersion, content.value, compact.value, reason.value)
    correcting.value = undefined
    await load()
    toast.add({ severity: 'success', summary: '记忆已更正', life: 2500 })
  } catch {
    viewState.value = 'conflict'
  }
}
</script>

<template>
  <section class="drawer-view" data-agent-memories-view>
    <header class="drawer-toolbar">
      <span class="drawer-title">记忆</span>
      <span class="count-badge">{{ memories.length }}</span>
      <button type="button" class="icon-btn" title="刷新" @click="load"><IconRefreshCw :size="14" /></button>
    </header>
    <Banner
      v-if="viewState === 'unavailable' && !bannerClosed"
      tone="danger"
      message="记忆列表读取失败"
      action-label="Retry"
      @action="load"
      @close="bannerClosed = true"
    />
    <Banner
      v-else-if="viewState === 'conflict' && !bannerClosed"
      tone="warning"
      message="记忆版本冲突，请重新读取后再操作"
      action-label="刷新"
      @action="load"
      @close="bannerClosed = true"
    />

    <div v-if="viewState === 'loading'" class="skel"><Skeleton v-for="n in 4" :key="n" height="32px" /></div>
    <template v-else>
      <section class="memory-section memory-section--candidates">
        <header class="memory-section__head">
          <span class="memory-section__eyebrow">候选</span>
          <span class="memory-section__count">{{ candidates.length }}</span>
          <span class="memory-section__caption">Agent 提取的待审核记忆片段。接受即写入长期记忆之前的复核队列。</span>
        </header>
        <p v-if="candidates.length === 0" class="empty-mini">没有待审核候选</p>
        <DataTable v-else :value="candidates" data-key="candidateId" size="small">
          <Column header="STATUS" style="width: 6rem">
            <template #body="{ data }">
              <StatusPill :tone="data.status === 'pending' ? 'warning' : data.status === 'accepted' ? 'success' : 'danger'" :label="data.status" dense />
            </template>
          </Column>
          <Column header="SUMMARY" field="summary" />
          <Column header="" style="width: 9rem">
            <template #body="{ data }">
              <template v-if="data.status === 'pending'">
                <button type="button" class="link" :data-agent-memory-candidate-accept="data.candidateId" @click="review(data, 'accepted')">接受</button>
                <button type="button" class="link" :data-agent-memory-candidate-reject="data.candidateId" @click="review(data, 'rejected')">拒绝</button>
              </template>
              <button
                v-else-if="data.status === 'accepted' && data.reviewId"
                type="button"
                class="link"
                :data-agent-memory-candidate-promote="data.candidateId"
                @click="promote(data)"
              >晋升</button>
            </template>
          </Column>
        </DataTable>
      </section>

      <section class="memory-section memory-section--durable">
        <header class="memory-section__head">
          <span class="memory-section__eyebrow">长期记忆</span>
          <span class="memory-section__count">{{ memories.length }}</span>
          <span class="memory-section__caption">已晋升的稳定记忆。Agent 决策时会读取，可以更正或作废。</span>
        </header>
        <p v-if="memories.length === 0" class="empty-mini">还没有长期记忆</p>
        <DataTable v-else :value="memories" data-key="memoryId" size="small" striped-rows>
        <Column header="TYPE" field="memoryType" />
        <Column header="STATUS" style="width: 5.5rem">
          <template #body="{ data }">
            <StatusPill :tone="data.status === 'active' ? 'agent' : 'danger'" :label="data.status" dense />
          </template>
        </Column>
        <Column header="CONTENT">
          <template #body="{ data }">{{ data.compactContent || data.content }}</template>
        </Column>
        <Column v-if="agentFlags.memoryCorrection" header="" style="width: 4rem">
          <template #body="{ data }">
            <button
              v-if="data.status === 'active'"
              type="button"
              class="link"
              :data-agent-memory-correct="data.memoryId"
              @click="startCorrect(data)"
            >更正</button>
          </template>
        </Column>
        </DataTable>
      </section>
    </template>

    <Dialog v-model:visible="correctOpen" modal header="更正记忆" :style="{ width: 'min(520px, 92vw)' }">
      <div class="create-form">
        <label>正文 <Textarea v-model="content" rows="5" data-agent-memory-correction-content /></label>
        <label>摘要 <InputText v-model="compact" data-agent-memory-correction-compact /></label>
        <label>原因 <InputText v-model="reason" data-agent-memory-correction-reason /></label>
        <button type="button" class="primary-btn" data-agent-memory-correction-confirm :disabled="!reason.trim()" @click="submitCorrect">提交更正</button>
      </div>
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
.icon-btn, .link { border: 0; background: transparent; color: var(--dp-accent); cursor: pointer; font: 700 12px var(--dp-font-body); }
.icon-btn { color: var(--dp-ink-soft); padding: 6px; display: inline-flex; }
.icon-btn:hover { color: var(--dp-ink); background: var(--dp-surface-muted); }
.primary-btn { border: 0; background: var(--dp-accent); color: var(--dp-text-inverse); height: 30px; padding: 0 12px; font: 600 12px var(--dp-font-body); cursor: pointer; }
.primary-btn:hover:not(:disabled) { background: var(--dp-accent-strong); }
.skel { display: grid; gap: 8px; padding: 12px; }

.memory-section {
  padding: 16px 14px 4px;
  border-bottom: 1px solid var(--dp-line);
}
.memory-section:last-of-type { border-bottom: 0; }
/* Both sections share the panel background; only the eyebrow color and
   a subtle left accent distinguish 候选 (warning) from 长期记忆 (agent). */
.memory-section { background: var(--dp-bg-panel); }
.memory-section--candidates { border-left: 2px solid var(--dp-warning); }
.memory-section--candidates .memory-section__eyebrow { color: var(--dp-warning); }
.memory-section--durable { border-left: 2px solid var(--dp-agent); }
.memory-section--durable .memory-section__eyebrow { color: var(--dp-agent-strong); }
.memory-section__head {
  display: grid;
  grid-template-columns: auto auto 1fr;
  align-items: center;
  gap: 8px;
  row-gap: 4px;
  margin-bottom: 10px;
}
.memory-section__eyebrow {
  font: 800 11px var(--dp-font-data); letter-spacing: 0.1em; text-transform: uppercase;
}
.memory-section__count {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 18px; padding: 0 6px;
  background: rgba(255, 255, 255, 0.6); color: var(--dp-ink);
  border: 1px solid var(--dp-line);
  font: 700 10px var(--dp-font-data); letter-spacing: 0.04em;
}
.memory-section__caption {
  grid-column: 1 / -1;
  font: 400 11px var(--dp-font-body); color: var(--dp-ink-soft); line-height: 1.5;
}
.empty-mini {
  padding: 12px 0; margin: 0;
  color: var(--dp-ink-faint);
  font: 500 12px var(--dp-font-body);
}

.create-form { display: grid; gap: 10px; }
.create-form label { display: grid; gap: 4px; font: 600 12px var(--dp-font-body); }
</style>
