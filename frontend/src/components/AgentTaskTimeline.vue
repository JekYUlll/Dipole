<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { agentTaskClient, type AgentTaskClient, type AgentTaskTimelineEvent } from '@/api/agentTasks'

const props = withDefaults(defineProps<{ taskId: string; client?: AgentTaskClient }>(), { client: undefined })
const client = computed(() => props.client ?? agentTaskClient)
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const events = ref<AgentTaskTimelineEvent[]>([])
const nextCursor = ref('')
const revision = ref<number | undefined>()
const loadingMore = ref(false)

onMounted(() => { void load() })

async function load() {
  state.value = 'loading'
  events.value = []
  nextCursor.value = ''
  try {
    if (client.value.getTimeline === undefined) throw new Error('Agent Task Timeline is unavailable')
    const page = await client.value.getTimeline(props.taskId)
    events.value = page.events
    nextCursor.value = page.nextCursor
    revision.value = page.revision
    state.value = 'ready'
  } catch {
    state.value = 'unavailable'
  }
}

async function loadMore() {
  if (loadingMore.value || !nextCursor.value || client.value.getTimeline === undefined) return
  loadingMore.value = true
  try {
    const page = await client.value.getTimeline(props.taskId, nextCursor.value)
    events.value.push(...page.events)
    nextCursor.value = page.nextCursor
    revision.value = page.revision
  } catch {
    nextCursor.value = ''
  } finally {
    loadingMore.value = false
  }
}

function eventLabel(event: AgentTaskTimelineEvent): string {
  const labels: Record<string, string> = {
    task: '任务', run: '运行', context_compile: '上下文', model_call: '模型调用',
    tool_invocation: '工具调用', approval: '审批', input_request: '补充信息', artifact: '产物', terminal: '结束',
  }
  return labels[event.kind] ?? '事件'
}

function eventTime(event: AgentTaskTimelineEvent): string {
  return new Date(event.occurredAtUnixMs).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function artifactRoute(event: AgentTaskTimelineEvent): { name: 'agent-artifact'; params: { artifactId: string } } | undefined {
  if (event.kind !== 'artifact' || event.artifactId === undefined) return undefined
  return { name: 'agent-artifact', params: { artifactId: event.artifactId } }
}
</script>

<template>
  <section class="timeline-shell" data-agent-task-timeline :data-state="state" aria-live="polite">
    <header class="timeline-header">
      <div>
        <p class="eyebrow">AGENT TASK / TIMELINE</p>
        <h2>执行轨迹</h2>
      </div>
      <span v-if="revision !== undefined" class="revision">REV {{ revision }}</span>
    </header>
    <div v-if="state === 'loading'" class="timeline-state">正在读取任务事件</div>
    <div v-else-if="state === 'unavailable'" class="timeline-state timeline-state-danger">
      <strong>时间线暂不可用</strong>
      <button type="button" class="text-action" data-agent-timeline-retry @click="load">重新读取</button>
    </div>
    <div v-else-if="events.length === 0" class="timeline-state">当前任务还没有可展示的事件</div>
    <ol v-else class="timeline-list">
      <li v-for="event in events" :key="event.eventId" class="timeline-item" :data-event-seq="event.eventSeq">
        <span class="timeline-dot" aria-hidden="true" />
        <div class="timeline-copy">
          <div class="timeline-meta"><strong>{{ eventLabel(event) }}</strong><time>{{ eventTime(event) }}</time></div>
          <p>{{ event.status }}</p>
          <small v-if="event.capabilityId">{{ event.capabilityId }}</small>
          <RouterLink v-if="artifactRoute(event)" class="artifact-link" :to="artifactRoute(event)!">查看 Artifact metadata</RouterLink>
        </div>
      </li>
    </ol>
    <button v-if="state === 'ready' && nextCursor" type="button" class="load-more" :disabled="loadingMore" data-agent-timeline-more @click="loadMore">
      {{ loadingMore ? '正在读取' : '读取更早事件' }}
    </button>
  </section>
</template>

<style scoped>
.timeline-shell { border: 1px solid var(--dp-line); border-radius: var(--dp-radius-md); padding: var(--dp-space-lg); background: var(--dp-surface); color: var(--dp-ink); font-family: var(--dp-font-body); }
.timeline-header { display: flex; align-items: start; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.eyebrow { margin: 0 0 .35rem; color: var(--dp-accent-strong); font: 700 .68rem/1.2 var(--dp-font-data); letter-spacing: .14em; }
h2 { margin: 0; color: var(--dp-ink); font: 700 1.2rem/1.25 var(--dp-font-display); }
.revision { color: var(--dp-ink-soft); font: .7rem/1.4 var(--dp-font-data); }
.timeline-state { display: flex; align-items: center; justify-content: space-between; min-height: 5rem; color: var(--dp-ink-soft); }
.timeline-state-danger { color: var(--dp-danger); }
.text-action, .load-more { border: 0; background: transparent; color: var(--dp-accent-strong); cursor: pointer; font: 600 .85rem/1.4 var(--dp-font-body); }
.timeline-list { position: relative; margin: 0; padding: .25rem 0 .25rem 1.25rem; list-style: none; }
.timeline-list::before { position: absolute; top: .7rem; bottom: .7rem; left: .28rem; width: 1px; background: var(--dp-line); content: ''; }
.timeline-item { position: relative; display: flex; gap: .8rem; padding: .65rem 0; }
.timeline-dot { position: absolute; left: -.99rem; top: .85rem; width: .55rem; height: .55rem; border: 2px solid var(--dp-surface); border-radius: 50%; background: var(--dp-accent); box-shadow: 0 0 0 1px var(--dp-accent); }
.timeline-copy { min-width: 0; flex: 1; }
.timeline-meta { display: flex; justify-content: space-between; gap: 1rem; color: var(--dp-ink); }
.timeline-meta time { color: var(--dp-ink-faint); font-size: .75rem; }
.timeline-copy p { margin: .2rem 0 0; color: var(--dp-ink-soft); font-size: .85rem; }
.timeline-copy small { display: block; margin-top: .3rem; color: var(--dp-accent-strong); font: .7rem/1.3 var(--dp-font-data); overflow-wrap: anywhere; }
.artifact-link { display: inline-block; margin-top: .45rem; color: var(--dp-accent-strong); font: 600 .78rem/1.3 var(--dp-font-body); }
.load-more { display: block; margin: .75rem auto 0; }
</style>
