<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { agentArtifactClient, type AgentArtifactClient, type AgentArtifactMetadata } from '@/api/agentArtifacts'

const props = withDefaults(defineProps<{ artifactId: string; client?: AgentArtifactClient }>(), { client: undefined })
const client = computed(() => props.client ?? agentArtifactClient)
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const artifact = ref<AgentArtifactMetadata>()

onMounted(() => { void load() })

async function load() {
  state.value = 'loading'
  artifact.value = undefined
  try {
    artifact.value = await client.value.get(props.artifactId)
    state.value = 'ready'
  } catch {
    state.value = 'unavailable'
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
        <h1>Artifact metadata</h1>
        <p class="subtitle">仅展示认证 owner 可读取的低敏运行产物信息。</p>
      </div>
      <span class="metadata-badge">METADATA ONLY</span>
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
        <p>DISCLOSURE BOUNDARY</p><h2>内容与下载保持关闭</h2><span>正文、对象键与下载不进入浏览器。</span>
      </section>
    </template>
  </section>
</template>

<style scoped>
.artifact-shell { box-sizing: border-box; width: min(100%, 56rem); min-height: 100vh; margin: 0 auto; padding: clamp(1.25rem, 4vw, 2.5rem); background: var(--dp-canvas); color: var(--dp-ink); font-family: var(--dp-font-body); }
.artifact-header { display: flex; justify-content: space-between; gap: 1rem; align-items: flex-start; margin-bottom: 1.5rem; }
.eyebrow, .artifact-type, .integrity p, .disclosure p { margin: 0 0 .45rem; color: var(--dp-accent-strong); font: 700 .7rem/1.25 var(--dp-font-data); letter-spacing: .1em; }
.artifact-header h1 { margin: 0; font: 800 clamp(1.7rem, 4vw, 2.4rem)/1.15 var(--dp-font-display); }
.subtitle { margin: .45rem 0 0; color: var(--dp-ink-soft); font-size: .9rem; }
.metadata-badge { padding: .5rem .7rem; border-radius: 99px; background: var(--dp-accent-soft); color: var(--dp-accent-strong); font: 700 .68rem/1 var(--dp-font-data); white-space: nowrap; }
.artifact-state, .artifact-summary, .metadata-grid > div, .integrity, .disclosure { border: 1px solid var(--dp-line); border-radius: var(--dp-radius-md); background: var(--dp-surface); }
.artifact-state { min-height: 7rem; padding: 1.25rem; display: flex; align-items: center; justify-content: space-between; gap: 1rem; color: var(--dp-ink-soft); }
.artifact-state p { margin: .4rem 0 0; }.artifact-state-danger strong { color: var(--dp-danger); } button { border: 0; background: transparent; color: var(--dp-accent-strong); font: 600 .9rem var(--dp-font-body); cursor: pointer; }
.artifact-summary { display: flex; gap: 1rem; align-items: center; padding: 1.25rem; }.type-tile { display: grid; place-items: center; width: 4.5rem; height: 4.5rem; border-radius: var(--dp-radius-sm); background: var(--dp-accent-soft); color: var(--dp-accent-strong); font: 700 .95rem var(--dp-font-data); }
.artifact-summary h2, .disclosure h2 { margin: 0; font: 750 1.25rem/1.3 var(--dp-font-display); }.artifact-summary p:last-child { margin: .35rem 0 0; color: var(--dp-ink-soft); font: .78rem var(--dp-font-data); }
.metadata-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .75rem; margin: 1rem 0; }.metadata-grid > div { padding: 1rem; }.metadata-grid dt { color: var(--dp-ink-faint); font: 700 .68rem var(--dp-font-data); letter-spacing: .08em; }.metadata-grid dd { margin: .55rem 0 0; overflow-wrap: anywhere; color: var(--dp-ink); font: .82rem var(--dp-font-data); }
.integrity { display: flex; flex-direction: column; gap: .3rem; padding: 1rem 1.2rem; background: var(--dp-accent-soft); }.integrity p { margin: 0; }.integrity strong { font: .86rem var(--dp-font-data); }.integrity span, .disclosure span { color: var(--dp-ink-soft); font-size: .82rem; }.disclosure { margin-top: 1rem; padding: 1rem 1.2rem; }.disclosure p { color: var(--dp-warning); margin-bottom: .45rem; }
@media (max-width: 560px) { .artifact-header { flex-direction: column; }.metadata-grid { grid-template-columns: 1fr; }.artifact-state { align-items: flex-start; flex-direction: column; }.artifact-summary { align-items: flex-start; }.type-tile { flex: 0 0 auto; } }
</style>
