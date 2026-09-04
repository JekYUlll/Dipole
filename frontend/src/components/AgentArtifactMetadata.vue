<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { agentArtifactClient, type AgentArtifactClient, type AgentArtifactContent, type AgentArtifactMetadata } from '@/api/agentArtifacts'
import { agentFlags } from '@/config/agentFlags'

const props = withDefaults(defineProps<{ artifactId: string; client?: AgentArtifactClient; embedded?: boolean }>(), {
  client: undefined,
  embedded: false,
})
const client = computed(() => props.client ?? agentArtifactClient)
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const artifact = ref<AgentArtifactMetadata>()
const contentState = ref<'idle' | 'loading' | 'ready' | 'unavailable' | 'unsupported'>('idle')
const content = ref<AgentArtifactContent>()
const isConversationDigest = computed(() => artifact.value?.artifactType === 'conversation_digest' && artifact.value.mediaType === 'text/markdown')
const inboxEnabled = agentFlags.artifacts

onMounted(() => { void load() })

async function load() {
  state.value = 'loading'
  artifact.value = undefined

  content.value = undefined
  contentState.value = 'idle'
  try {
    artifact.value = await client.value.get(props.artifactId)
    state.value = 'ready'
    await loadContent()
  } catch {
    state.value = 'unavailable'
  }
}

async function loadContent() {
  const current = artifact.value
  content.value = undefined
  if (!current || !isConversationDigest.value) {
    contentState.value = 'unsupported'
    return
  }
  contentState.value = 'loading'
  try {
    const value = await client.value.getContent(current.artifactId)
    if (value.artifactId !== current.artifactId || value.mediaType !== current.mediaType) throw new Error('Artifact content binding changed')
    content.value = value
    contentState.value = 'ready'
  } catch {
    contentState.value = 'unavailable'
  }
}

function timestamp(value: number): string {
  return new Date(value).toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'short' })
}

function shortID(value: string): string { return `${value.slice(0, 12)}...${value.slice(-8)}` }
</script>

<template>
  <section class="artifact-shell" data-agent-artifact :data-agent-artifact-state="state" aria-live="polite">
    <header class="artifact-header">
      <div>
        <p class="eyebrow">TASK ARTIFACT / OWNER VIEW</p>
        <h1>Artifact digest</h1>
        <p class="subtitle">认证 owner 可读取任务摘要；下载与对象存储信息保持关闭。</p>
      </div>
      <div class="header-actions">
        <RouterLink
          v-if="inboxEnabled && !props.embedded"
          class="inbox-link"
          :to="{ path: '/', query: { agent: '1', view: 'artifacts' } }"
          data-agent-artifact-inbox
        >返回产物列表 →</RouterLink>
        <span class="metadata-badge">OWNER READ</span>
      </div>
    </header>

    <div v-if="state === 'loading'" class="artifact-state" role="status">正在读取 Artifact metadata</div>
    <div v-else-if="state === 'unavailable'" class="artifact-state artifact-state-danger" role="alert">
      <div><strong>Artifact metadata 暂不可用</strong><p>已清空旧数据，避免展示未经确认的产物信息。</p></div>
      <button type="button" data-agent-artifact-retry @click="load">重新读取</button>
    </div>
    <template v-else-if="artifact">
      <article class="artifact-summary">
        <div class="type-tile">{{ artifact.mediaType.split('/')[1]?.toUpperCase() }}</div>
        <div><p class="artifact-type">{{ artifact.artifactType }} / VERSION {{ artifact.version }}</p><h2>{{ artifact.title }}</h2><p>{{ artifact.mediaType }} · {{ artifact.sizeBytes.toLocaleString('zh-CN') }} B</p></div>
      </article>
      <dl class="metadata-grid">
        <div><dt>Artifact ID</dt><dd>{{ shortID(artifact.artifactId) }}</dd></div>
        <div><dt>Task</dt><dd>{{ artifact.taskId }}</dd></div>
        <div><dt>Run</dt><dd>{{ artifact.runId }}</dd></div>
        <div><dt>创建时间</dt><dd>{{ timestamp(artifact.createdAtUnixMs) }}</dd></div>
      </dl>
      <section class="integrity" aria-label="Artifact content address">
        <p>CONTENT ADDRESS</p><strong>sha256: {{ shortID(artifact.contentSha256) }}</strong><span>内容寻址 ID 只用于定位可披露 metadata。</span>
      </section>
      <section class="disclosure" aria-label="Artifact disclosure boundary">
        <p>DISCLOSURE BOUNDARY</p><h2>任务摘要可读</h2><span>仅认证 owner 的 Markdown 摘要进入阅读区；对象键、Metadata JSON 与下载保持关闭。</span>
      </section>
      <section v-if="isConversationDigest" class="digest" aria-label="Agent digest" :data-agent-artifact-content-state="contentState">
        <div class="digest-header"><div><p>VERIFIED DIGEST</p><h2>Conversation digest</h2></div><span>{{ contentState === 'ready' ? 'READY' : 'READ ONLY' }}</span></div>
        <div v-if="contentState === 'loading'" class="digest-state" role="status">正在读取已验证摘要</div>
        <div v-else-if="contentState === 'unavailable'" class="digest-state digest-state-danger" role="alert">
          <div><strong>摘要正文暂不可用</strong><p>已确认的 Artifact metadata 保持可见。</p></div>
          <button type="button" data-agent-artifact-content-retry @click="loadContent">重新读取摘要</button>
        </div>
        <pre v-else-if="contentState === 'ready' && content" class="digest-body">{{ content.content }}</pre>
      </section>
      <section v-else class="digest digest-unsupported" data-agent-artifact-content-state="unsupported" aria-label="Artifact digest unavailable">
        <p>DISPLAYABLE DIGEST UNAVAILABLE</p><h2>该 Artifact 不提供可读摘要</h2><span>当前阅读区仅支持已验证的 conversation digest；metadata 继续可用。</span>
      </section>
    </template>
  </section>
</template>

<style scoped>
.artifact-shell { box-sizing: border-box; width: min(100%, 62rem); min-height: 100vh; margin: 0 auto; padding: clamp(1.25rem, 4vw, 2.5rem); background: var(--dp-canvas); color: var(--dp-ink); font-family: var(--dp-font-body); }
.artifact-header { display: flex; justify-content: space-between; gap: 1rem; align-items: flex-start; margin-bottom: 1.5rem; }
.eyebrow, .artifact-type, .integrity p, .disclosure p { margin: 0 0 .45rem; color: var(--dp-rail); font: 700 .7rem/1.25 var(--dp-font-data); letter-spacing: .1em; }
.artifact-header h1 { margin: 0; font: 800 clamp(1.7rem, 4vw, 2.4rem)/1.15 var(--dp-font-display); }
.subtitle { margin: .45rem 0 0; color: var(--dp-ink-soft); font-size: .9rem; }
.header-actions { display: flex; align-items: center; gap: .75rem; }
.inbox-link { color: var(--dp-accent); font: 700 .72rem var(--dp-font-body); text-decoration: none; white-space: nowrap; }
.metadata-badge { padding: .5rem .7rem; border-radius: 99px; background: var(--dp-agent-soft); color: var(--dp-rail); font: 700 .68rem/1 var(--dp-font-data); white-space: nowrap; }
.artifact-state, .artifact-summary, .metadata-grid > div, .integrity, .disclosure, .digest { border: 1px solid var(--dp-line); border-radius: var(--dp-radius-md); background: var(--dp-surface); }
.artifact-state { min-height: 7rem; padding: 1.25rem; display: flex; align-items: center; justify-content: space-between; gap: 1rem; color: var(--dp-ink-soft); }
.artifact-state p { margin: .4rem 0 0; }.artifact-state-danger strong { color: var(--dp-danger); } button { border: 0; background: transparent; color: var(--dp-rail); font: 600 .9rem var(--dp-font-body); cursor: pointer; }
.artifact-summary { display: flex; gap: 1rem; align-items: center; padding: 1.25rem; }.type-tile { display: grid; place-items: center; width: 4.5rem; height: 4.5rem; border-radius: var(--dp-radius-sm); background: var(--dp-agent-soft); color: var(--dp-rail); font: 700 .95rem var(--dp-font-data); }
.artifact-summary h2, .disclosure h2 { margin: 0; font: 750 1.25rem/1.3 var(--dp-font-display); }.artifact-summary p:last-child { margin: .35rem 0 0; color: var(--dp-ink-soft); font: .78rem var(--dp-font-data); }
.metadata-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .75rem; margin: 1rem 0; }.metadata-grid > div { padding: 1rem; }.metadata-grid dt { color: var(--dp-ink-faint); font: 700 .68rem var(--dp-font-data); letter-spacing: .08em; }.metadata-grid dd { margin: .55rem 0 0; overflow-wrap: anywhere; color: var(--dp-ink); font: .82rem var(--dp-font-data); }
.integrity { display: flex; flex-direction: column; gap: .3rem; padding: 1rem 1.2rem; background: var(--dp-agent-soft); }.integrity p { margin: 0; }.integrity strong { font: .86rem var(--dp-font-data); }.integrity span, .disclosure span, .digest-unsupported span { color: var(--dp-ink-soft); font-size: .82rem; }.disclosure { margin-top: 1rem; padding: 1rem 1.2rem; }.disclosure p, .digest-unsupported p { color: var(--dp-warning); margin-bottom: .45rem; }
.digest { margin-top: 1rem; overflow: hidden; }.digest-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; padding: 1rem 1.2rem; border-bottom: 1px solid var(--dp-line); background: linear-gradient(120deg, var(--dp-agent-soft), var(--dp-surface)); }.digest-header p { margin: 0 0 .35rem; color: var(--dp-rail); font: 700 .68rem var(--dp-font-data); letter-spacing: .1em; }.digest-header h2, .digest-unsupported h2 { margin: 0; font: 750 1.15rem/1.25 var(--dp-font-display); }.digest-header span { border: 1px solid var(--dp-rail); border-radius: 99px; padding: .3rem .5rem; color: var(--dp-rail); font: 700 .64rem var(--dp-font-data); letter-spacing: .06em; }.digest-body { box-sizing: border-box; width: 100%; min-height: 14rem; margin: 0; padding: 1.25rem; overflow-x: auto; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--dp-ink); font: .94rem/1.75 var(--dp-font-body); }.digest-state { min-height: 8rem; padding: 1.25rem; display: flex; align-items: center; justify-content: space-between; gap: 1rem; color: var(--dp-ink-soft); }.digest-state p { margin: .4rem 0 0; }.digest-state-danger strong { color: var(--dp-danger); }.digest-unsupported { padding: 1rem 1.2rem; }
@media (max-width: 560px) { .artifact-header { flex-direction: column; }.metadata-grid { grid-template-columns: 1fr; }.artifact-state { align-items: flex-start; flex-direction: column; }.artifact-summary { align-items: flex-start; }.type-tile { flex: 0 0 auto; } }
</style>
