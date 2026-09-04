<template>
  <section class="inbox-shell" :class="{ 'is-embedded': embedded }" :data-agent-artifact-inbox-state="viewState" :aria-busy="busy">
    <aside v-if="!embedded" class="control-rail" aria-label="Agent control navigation">
      <div class="brand"><span />DIPOLE</div>
      <p class="rail-kicker">AGENT CONTROL</p>
      <RouterLink v-if="nav.definitions" class="rail-item" :to="{ name: 'agent-definitions' }">▣ <span>Agent 定义</span></RouterLink>
      <div v-else class="rail-item">▣ <span>Agent 定义</span></div>
      <RouterLink v-if="nav.subscriptions" class="rail-item" :to="{ name: 'agent-subscriptions' }">⌁ <span>事件订阅</span></RouterLink>
      <div v-else class="rail-item">⌁ <span>事件订阅</span></div>
      <RouterLink v-if="nav.taskRun" class="rail-item" :to="nav.taskRun">☷ <span>任务运行</span></RouterLink>
      <div v-else class="rail-item">☷ <span>任务运行</span></div>
      <div class="rail-active">▦ <span>任务产物</span></div>
      <p class="rail-boundary">OWNER INBOX<br>METADATA ONLY<br>CONTENT STAYS ON DIGEST</p>
    </aside>

    <main class="inbox-main">
      <header class="page-header">
        <div>
          <p class="eyebrow">OWNER VIEW / ARTIFACT INBOX</p>
          <h1>任务产物</h1>
          <p class="subtitle">按创建时间列出当前账号任务的低敏 metadata。正文只在摘要页按 digest 限制读取。</p>
        </div>
      </header>

      <div v-if="viewState === 'loading'" class="state-card" role="status">
        <span class="spinner" /><p class="state-code">LOADING</p><h2>正在读取产物</h2><p>只显示当前认证 principal 的权威 metadata。</p>
      </div>
      <div v-else-if="viewState === 'unavailable'" class="state-card danger" role="alert">
        <p class="state-code">UNAVAILABLE</p><h2>产物收件箱暂时不可用</h2><p>已清空旧记录，避免展示未经确认的缓存状态。</p>
        <button data-agent-artifact-inbox-retry class="text-action danger-text" @click="load(true)">重新确认 →</button>
      </div>
      <div v-else-if="artifacts.length === 0" class="state-card" role="status">
        <p class="state-code success">EMPTY</p><h2>还没有产物</h2><p>任务完成后带 metadata 的 Artifact 会出现在这里。</p>
      </div>

      <template v-else>
        <div class="list-heading"><h2>ARTIFACT RECORDS&nbsp; {{ String(artifacts.length).padStart(2, '0') }}</h2><span>CREATED DESC ↓</span></div>
        <div class="artifact-list">
          <article
            v-for="item in artifacts"
            :key="item.artifactId"
            class="artifact-card"
            :data-agent-artifact-id="item.artifactId"
          >
            <div class="card-top">
              <div>
                <h3>{{ item.title }}</h3>
                <p class="mono">{{ item.artifactType }} · {{ item.mediaType }} · V{{ item.version }} · {{ item.sizeBytes.toLocaleString('zh-CN') }} B</p>
                <p class="mono">{{ item.taskId }} · {{ timestamp(item.createdAtUnixMs) }}</p>
              </div>
            </div>
            <div class="card-actions">
              <RouterLink class="text-action" :to="{ name: 'agent-artifact', params: { artifactId: item.artifactId } }" data-agent-artifact-inbox-open>查看摘要 →</RouterLink>
              <RouterLink
                v-if="nav.timeline"
                class="text-action"
                :to="{ name: 'agent-task-timeline', params: { taskId: item.taskId } }"
                data-agent-artifact-inbox-timeline
              >查看时间线 →</RouterLink>
            </div>
          </article>
          <button v-if="nextCursor" class="load-more" :disabled="busy" data-agent-artifact-inbox-more @click="loadMore">加载下一页 →</button>
        </div>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { agentArtifactClient, type AgentArtifactClient, type AgentArtifactMetadata } from '@/api/agentArtifacts'
import { agentFlags, agentTaskRunTarget } from '@/config/agentFlags'

const props = withDefaults(defineProps<{ client?: AgentArtifactClient; embedded?: boolean }>(), {
  client: () => agentArtifactClient,
  embedded: false,
})
const { embedded } = props
const nav = {
  definitions: agentFlags.definitions,
  subscriptions: agentFlags.subscriptions,
  taskRun: agentTaskRunTarget(),
  timeline: agentFlags.timeline,
}
type ViewState = 'loading' | 'ready' | 'unavailable'
const viewState = ref<ViewState>('loading')
const artifacts = ref<AgentArtifactMetadata[]>([])
const nextCursor = ref('')
const busy = computed(() => viewState.value === 'loading')

onMounted(() => load(false))

async function load(reset: boolean) {
  viewState.value = 'loading'
  if (reset) { artifacts.value = []; nextCursor.value = '' }
  try {
    const list = props.client.list
    if (list === undefined) throw new Error('Agent Artifact catalog is unavailable')
    const page = await list('', 50)
    artifacts.value = page.artifacts
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    artifacts.value = []
    nextCursor.value = ''
    viewState.value = 'unavailable'
  }
}

async function loadMore() {
  if (!nextCursor.value || busy.value || props.client.list === undefined) return
  viewState.value = 'loading'
  try {
    const page = await props.client.list(nextCursor.value, 50)
    const seen = new Set(artifacts.value.map(item => item.artifactId))
    if (page.artifacts.some(item => seen.has(item.artifactId))) throw new Error('duplicate Agent Artifact inbox page')
    artifacts.value.push(...page.artifacts)
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    nextCursor.value = ''
    viewState.value = 'unavailable'
  }
}

function timestamp(value: number): string {
  return new Date(value).toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'short' })
}
</script>

<style scoped>
.inbox-shell{--ink:var(--dp-ink);--muted:var(--dp-ink-soft);--line:var(--dp-line);--panel:var(--dp-surface);--app:var(--dp-canvas);--rail:var(--dp-rail);min-height:100vh;background:var(--app);color:var(--ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}
.control-rail{background:var(--rail);color:var(--dp-ink-faint);padding:34px 26px;display:flex;flex-direction:column;gap:20px}
.brand{font:800 19px var(--dp-font-display);letter-spacing:.12em;display:flex;align-items:center;gap:12px}
.brand>span{width:12px;height:12px;border-radius:50%;background:var(--dp-agent)}
.rail-kicker,.rail-boundary,.mono,.state-code{font-family:var(--dp-font-data)}
.rail-kicker{font-size:10px;color:var(--dp-ink-faint);letter-spacing:.12em;margin-top:8px}
.rail-active,.rail-item{border-radius:10px;padding:12px 14px;display:flex;gap:10px;align-items:center;font-size:13px}
.rail-active{background:var(--dp-rail-soft);color:var(--dp-text-inverse);font-weight:700}
.rail-item{color:var(--dp-ink-faint)}
.rail-boundary{margin-top:auto;font-size:9px;line-height:1.9;color:var(--dp-ink-faint)}
.inbox-main{padding:36px 42px 64px;max-width:1100px;width:100%;margin:auto}
.page-header{display:flex;justify-content:space-between;align-items:center;gap:24px}
.eyebrow{color:var(--dp-rail);font:700 10px var(--dp-font-data);letter-spacing:.1em}
.page-header h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:7px 0}
.subtitle{color:var(--muted);font-size:13px}
.text-action,.load-more{border:0;background:transparent;color:var(--dp-accent);font:700 11px inherit;cursor:pointer;text-decoration:none}
.state-card{max-width:720px;background:var(--panel);border:1px solid var(--line);border-radius:18px;padding:32px;margin:70px auto}
.state-card h2{font:700 25px var(--dp-font-display);margin:8px 0}
.state-card>p:not(.state-code){color:var(--muted)}
.state-card.danger{background:var(--dp-danger-soft);border-left:5px solid var(--dp-danger)}
.state-code{font-size:10px;font-weight:700;color:var(--dp-danger)}
.state-code.success{color:var(--dp-rail)}
.spinner{display:block;width:24px;height:24px;border:3px solid var(--line);border-top-color:var(--dp-rail);border-radius:50%;animation:spin .8s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
.list-heading{display:flex;justify-content:space-between;align-items:center;margin:26px 0 14px}
.list-heading h2{font:700 11px var(--dp-font-data);color:var(--dp-ink-soft);letter-spacing:.08em}
.artifact-list{display:grid;gap:14px}
.artifact-card{background:var(--panel);border:1px solid var(--line);border-radius:15px;padding:20px}
.card-top h3{font:800 17px var(--dp-font-display);margin:0;max-width:640px}
.mono{font-size:9px;color:var(--dp-ink-faint);margin-top:6px}
.card-actions{display:flex;gap:14px;margin-top:16px}
.danger-text{color:var(--dp-danger)}
a.rail-item,a.rail-active{text-decoration:none;color:inherit}
@media(max-width:900px){.inbox-shell{grid-template-columns:1fr}.control-rail{display:none}.inbox-main{padding:26px 20px 60px}.page-header{align-items:flex-start;flex-direction:column}}
.inbox-shell.is-embedded{grid-template-columns:1fr;min-height:auto}
</style>
