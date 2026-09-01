<template>
  <section class="definition-shell" :aria-busy="busy" data-agent-definition-state="">
    <aside class="control-rail" aria-label="Agent control navigation">
      <div class="brand"><img class="brand-mark" :src="agentMark" alt="" aria-hidden="true" /><span>DIPOLE</span></div>
      <p class="rail-kicker">AGENT CONTROL</p>
      <div class="rail-active">◈ <span>Agent 定义</span></div>
      <div class="rail-item">⌁ <span>事件订阅</span></div>
      <div class="rail-item">☷ <span>任务运行</span></div>
      <div class="rail-item">♢ <span>审批记录</span></div>
      <p class="rail-boundary">OWNER GOVERNANCE<br>READ ONLY CATALOG<br>RUNTIME: DIRECT TARGET</p>
    </aside>

    <main class="definition-main">
      <div class="mobile-brandbar"><img class="brand-mark" :src="agentMark" alt="" aria-hidden="true" /><span>Agent 定义</span><small>READ ONLY</small></div>
      <header class="page-header">
        <div>
          <p class="eyebrow">OWNER VIEW / ACTIVE DEFINITIONS</p>
          <h1>Agent 定义</h1>
          <p class="subtitle">查看当前认证用户可选的 active Definition；版本与会话 scope 由 Core 权威返回。</p>
        </div>
        <div class="catalog-status">◈ <strong>CORE AUTHORITY</strong></div>
      </header>

      <div class="trust-notice" role="note">
        <strong>READ ONLY CATALOG</strong>
        <span>目录只提供订阅配置的候选项，不会启动 Runtime，也不会授予额外能力。</span>
      </div>

      <div v-if="viewState === 'loading'" class="state-card" role="status">
        <p class="state-code">LOADING</p><h2>正在读取 Agent 定义</h2><p>只显示当前认证 principal 的 active 版本。</p>
      </div>
      <div v-else-if="viewState === 'unavailable'" class="state-card danger" role="alert">
        <p class="state-code">UNAVAILABLE</p><h2>定义目录暂时不可用</h2><p>已清空旧候选，避免使用未经确认的版本创建控制记录。</p>
        <button class="text-action danger-text" data-agent-definition-retry @click="load(true)">重新确认 →</button>
      </div>
      <div v-else-if="definitions.length === 0" class="state-card" role="status">
        <p class="state-code success">EMPTY</p><h2>当前没有可选定义</h2><p>Core 尚未返回当前 owner 可用的 active Definition。</p>
      </div>
      <template v-else>
        <div class="list-heading"><h2>ACTIVE DEFINITIONS&nbsp; {{ String(definitions.length).padStart(2, '0') }}</h2><span>VERSIONED · OWNER SCOPED</span></div>
        <div class="definition-list">
          <article v-for="definition in definitions" :key="`${definition.definitionId}:${definition.version}`" class="definition-card" :data-agent-definition-id="definition.definitionId">
            <div class="card-top">
              <div>
                <h2>{{ definition.agentId === 'UAI' ? 'Project Guardian' : definition.agentId }}</h2>
                <p class="mono">{{ definition.definitionId }} · VERSION {{ definition.version }}</p>
              </div>
              <span class="status-pill"><i />ACTIVE</span>
            </div>
            <div class="scope-block">
              <span class="field-label">CONVERSATION SCOPE</span>
              <div class="scope-list"><span v-for="scope in definition.conversationScopes" :key="scope" class="scope-chip">{{ scope }}</span></div>
            </div>
            <div class="card-bottom mono">
              <span>VALID FROM {{ formatDate(definition.validFromUnixMs) }}</span>
              <span>{{ definition.expiresAtUnixMs ? `EXPIRES ${formatDate(definition.expiresAtUnixMs)}` : 'NO EXPIRY' }}</span>
            </div>
          </article>
          <button v-if="nextCursor" class="load-more" data-agent-definition-load-more :disabled="busy" @click="loadMore">加载下一页 →</button>
        </div>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { agentDefinitionCatalogClient, type AgentDefinitionCatalogClient, type AgentDefinitionCatalogItem } from '@/api/agentDefinitions'
import agentMark from '../../../docs/images/dipole-v3-agent-mark-traced.svg'

const props = withDefaults(defineProps<{ client?: AgentDefinitionCatalogClient }>(), { client: () => agentDefinitionCatalogClient })
const definitions = ref<AgentDefinitionCatalogItem[]>([])
const nextCursor = ref('')
const viewState = ref<'loading' | 'ready' | 'unavailable'>('loading')
const busy = computed(() => viewState.value === 'loading')

onMounted(() => { void load(true) })

async function load(reset: boolean): Promise<void> {
  viewState.value = 'loading'
  if (reset) { definitions.value = []; nextCursor.value = '' }
  try {
    const page = await props.client.list(reset ? '' : nextCursor.value, 50)
    definitions.value = reset ? page.definitions : [...definitions.value, ...page.definitions]
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    definitions.value = []
    nextCursor.value = ''
    viewState.value = 'unavailable'
  }
}

function loadMore(): void { if (nextCursor.value && !busy.value) void load(false) }
function formatDate(unixMs: number): string { return new Date(unixMs).toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }) }
</script>

<style scoped>
.definition-shell { --paper:var(--dp-canvas); --cream:var(--dp-surface); --ink:var(--dp-ink); --muted:var(--dp-ink-soft); --line:var(--dp-line); --forest:var(--dp-rail); min-height:100vh; display:grid; grid-template-columns:236px 1fr; color:var(--ink); background:radial-gradient(circle at 78% 0%,var(--dp-accent-soft),transparent 28%),var(--paper); font-family:var(--dp-font-body) }
.control-rail { padding:30px 22px; color:var(--dp-text-inverse); background:var(--forest) }.brand { display:flex; align-items:center; gap:10px; font:800 14px var(--dp-font-data); letter-spacing:.18em }.brand-mark { width:32px; height:32px; object-fit:contain; border-radius:9px }.rail-kicker,.rail-label,.eyebrow,.state-code,.field-label { margin:0; font:800 10px var(--dp-font-data); letter-spacing:.16em }.rail-kicker { margin-top:52px; color:var(--dp-accent-soft) }.rail-active,.rail-item { margin-top:18px; padding:11px 10px; border-radius:var(--dp-radius-sm); font-size:13px }.rail-active { color:var(--dp-text-inverse); background:color-mix(in srgb,var(--dp-accent) 18%,transparent) }.rail-item { color:var(--dp-ink-faint) }.rail-boundary { margin-top:100px; color:var(--dp-ink-faint); font:700 9px/1.8 var(--dp-font-data); letter-spacing:.12em }
.definition-main { min-width:0; padding:clamp(24px,5vw,70px) clamp(18px,5vw,76px) }.mobile-brandbar { display:none }.page-header { display:flex; align-items:end; justify-content:space-between; gap:24px; max-width:1040px; margin:0 auto 26px }.eyebrow { color:var(--dp-accent-strong) }.page-header h1 { margin:7px 0 8px; font:500 clamp(32px,5vw,58px)/1 var(--dp-font-display); letter-spacing:-.05em }.subtitle { max-width:650px; margin:0; color:var(--muted); font-size:14px; line-height:1.7 }.catalog-status { align-self:start; padding:10px 13px; border:1px solid var(--line); border-radius:999px; color:var(--dp-accent-strong); font:700 10px var(--dp-font-data); letter-spacing:.1em; white-space:nowrap }.trust-notice,.state-card,.definition-list { max-width:1040px; margin-left:auto; margin-right:auto }.trust-notice { display:flex; gap:14px; align-items:baseline; padding:14px 18px; border-left:4px solid var(--dp-accent); color:var(--muted); background:color-mix(in srgb,var(--dp-accent-soft) 60%,var(--cream)); font-size:12px }.trust-notice strong { color:var(--dp-accent-strong); font:800 10px var(--dp-font-data); letter-spacing:.12em; white-space:nowrap }.state-card { margin-top:30px; padding:36px; border:1px solid var(--line); border-radius:4px 22px 4px 4px; background:var(--cream); box-shadow:0 20px 55px color-mix(in srgb,var(--forest) 10%,transparent) }.state-card h2 { margin:8px 0; font:500 26px var(--dp-font-display) }.state-card p:not(.state-code) { color:var(--muted); font-size:13px }.state-code { color:var(--dp-accent-strong) }.danger { border-left:5px solid var(--dp-danger) }.danger .state-code,.danger-text { color:var(--dp-danger) }.success { color:var(--dp-accent-strong) }.text-action,.load-more { border:0; color:var(--dp-accent-strong); background:transparent; font:800 12px var(--dp-font-data); cursor:pointer }.list-heading { max-width:1040px; margin:32px auto 12px; display:flex; justify-content:space-between; align-items:center; gap:15px }.list-heading h2 { margin:0; font:800 12px var(--dp-font-data); letter-spacing:.13em }.list-heading span { color:var(--muted); font:700 10px var(--dp-font-data) }.definition-list { display:grid; grid-template-columns:repeat(auto-fit,minmax(290px,1fr)); gap:14px }.definition-card { padding:24px; border:1px solid var(--line); border-radius:5px 20px 5px 5px; background:var(--cream); box-shadow:0 15px 35px color-mix(in srgb,var(--forest) 8%,transparent) }.card-top { display:flex; justify-content:space-between; gap:12px; align-items:start }.card-top h2 { margin:0 0 7px; font:500 24px var(--dp-font-display) }.mono { color:var(--muted); font:700 10px var(--dp-font-data); letter-spacing:.05em; overflow-wrap:anywhere }.status-pill { display:inline-flex; align-items:center; gap:5px; padding:5px 8px; color:var(--dp-accent-strong); border:1px solid color-mix(in srgb,var(--dp-accent) 55%,var(--line)); border-radius:999px; font:800 9px var(--dp-font-data); letter-spacing:.1em }.status-pill i { width:5px; height:5px; border-radius:50%; background:currentColor }.scope-block { margin-top:26px; padding:14px; border:1px solid var(--line); background:color-mix(in srgb,var(--dp-muted-surface) 70%,transparent) }.field-label { color:var(--muted); font-size:9px }.scope-list { display:flex; flex-wrap:wrap; gap:7px; margin-top:10px }.scope-chip { padding:6px 8px; color:var(--ink); border:1px solid var(--line); border-radius:4px; background:var(--paper); font:700 11px var(--dp-font-data) }.card-bottom { display:flex; justify-content:space-between; gap:12px; margin-top:18px; padding-top:14px; border-top:1px solid var(--line) }.load-more { grid-column:1/-1; justify-self:center; padding:14px 18px }.load-more:disabled { opacity:.55; cursor:not-allowed }
@media (max-width:900px) { .definition-shell { display:block }.control-rail { display:none }.definition-main { padding:0 14px 30px }.mobile-brandbar { display:flex; align-items:center; gap:10px; min-height:64px; margin:0 -14px 26px; padding:10px 18px; color:var(--dp-text-inverse); background:var(--forest) }.mobile-brandbar .brand-mark { width:34px; height:34px }.mobile-brandbar span { font:800 12px var(--dp-font-data); letter-spacing:.1em }.mobile-brandbar small { margin-left:auto; color:var(--dp-accent-soft); font:800 9px var(--dp-font-data); letter-spacing:.12em }.page-header { align-items:start; margin-bottom:20px }.catalog-status { display:none }.trust-notice { display:block; padding:14px }.trust-notice strong { display:block; margin-bottom:7px }.definition-list { grid-template-columns:1fr }.definition-card { padding:20px }.list-heading { align-items:start; flex-direction:column; gap:6px }.card-bottom { flex-direction:column; gap:6px } }
</style>
