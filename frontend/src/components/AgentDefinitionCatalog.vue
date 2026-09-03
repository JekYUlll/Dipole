<template>
  <section class="definition-shell" :data-agent-definition-state="viewState" :aria-busy="busy">
    <aside class="control-rail" aria-label="Agent control navigation">
      <div class="brand"><span />DIPOLE</div>
      <p class="rail-kicker">AGENT CONTROL</p>
      <RouterLink v-if="nav.definitions" class="rail-active" :to="{ name: 'agent-definitions' }">▣ <span>Agent 定义</span></RouterLink>
      <div v-else class="rail-active">▣ <span>Agent 定义</span></div>
      <RouterLink v-if="nav.subscriptions" class="rail-item" :to="{ name: 'agent-subscriptions' }">⌁ <span>事件订阅</span></RouterLink>
      <div v-else class="rail-item">⌁ <span>事件订阅</span></div>
      <RouterLink v-if="nav.taskRun" class="rail-item" :to="nav.taskRun">☷ <span>任务运行</span></RouterLink>
      <div v-else class="rail-item">☷ <span>任务运行</span></div>
      <div class="rail-item">♢ <span>审批记录</span></div>
      <p class="rail-boundary">CATALOG ONLY<br>RUNTIME: DISABLED<br>OWNER SCOPED</p>
    </aside>

    <main class="definition-main">
      <header class="page-header">
        <div>
          <p class="eyebrow">OWNER VIEW / DEFINITION AUTHORITY</p>
          <h1>Agent 定义</h1>
          <p class="subtitle">只展示当前认证 principal 可用的精确版本与会话范围。</p>
        </div>
        <div class="header-actions">
          <button v-if="canCreate" data-agent-definition-create class="create-button" type="button" :disabled="busy || creating" @click="createProfile">
            {{ creating ? '正在创建' : '创建订阅回复定义' }}
          </button>
          <div class="catalog-status">◌ <strong>CATALOG + PROFILE</strong></div>
        </div>
      </header>

      <div class="boundary-notice" role="note">
        <strong>RUNTIME DISABLED</strong>
        <span>浏览目录不会启用模型或 Tool。创建只写入当前账号的 subscription_autoreply profile，订阅仍要单独确认。</span>
      </div>
      <p v-if="createError" class="form-error" role="alert">{{ createError }}</p>

      <div v-if="viewState === 'loading'" class="state-card" role="status">
        <span class="spinner" /><p class="state-code">LOADING</p><h2>正在读取 Agent 定义</h2><p>目录仅从认证后的 Core 权威响应构建。</p>
      </div>
      <div v-else-if="viewState === 'unavailable'" class="state-card danger" role="alert">
        <p class="state-code">UNAVAILABLE</p><h2>Definition 目录暂时不可用</h2><p>已清空旧目录，避免选择或展示未经确认的版本。</p>
        <button data-agent-definition-retry class="text-action danger-text" @click="load(true)">重新确认 →</button>
      </div>
      <div v-else-if="definitions.length === 0" class="state-card" role="status">
        <p class="state-code success">EMPTY</p><h2>还没有可用 Definition</h2><p>当前账号没有处于有效期内的 Agent Definition version。</p>
      </div>

      <template v-else>
        <div class="list-heading"><h2>AVAILABLE DEFINITIONS&nbsp; {{ String(definitions.length).padStart(2, '0') }}</h2><span>EXACT VERSION ↓</span></div>
        <div class="content-grid">
          <div class="definition-list">
            <article v-for="item in definitions" :key="identity(item)" class="definition-card" :class="{ expired: expired(item) }" :data-agent-definition-id="item.definitionId">
              <div class="card-top">
                <div>
                  <h3>{{ agentLabel(item.agentId) }}</h3>
                  <p class="mono">{{ item.definitionId }}&nbsp; /&nbsp; VERSION {{ String(item.version).padStart(2, '0') }}</p>
                </div>
                <span class="status-pill" :class="expired(item) ? 'status-expired' : 'status-active'"><i />{{ expired(item) ? 'EXPIRED' : 'ACTIVE / CATALOG' }}</span>
              </div>
              <div class="scope-section">
                <span class="scope-label">CONVERSATION SCOPE</span>
                <div class="scope-chips"><span v-for="scope in item.conversationScopes" :key="scope" class="scope-chip">{{ scope }}</span></div>
              </div>
              <dl class="definition-meta">
                <div><dt>VALID FROM</dt><dd>{{ timestamp(item.validFromUnixMs) }}</dd></div>
                <div><dt>EXPIRES AT</dt><dd>{{ item.expiresAtUnixMs === undefined ? 'NO EXPIRY' : timestamp(item.expiresAtUnixMs) }}</dd></div>
              </dl>
            </article>
            <button v-if="nextCursor" data-agent-definition-more class="load-more" :disabled="busy" @click="loadMore">加载下一页 →</button>
          </div>

          <aside class="authority-panel">
            <p class="eyebrow">PINNED AUTHORITY</p>
            <h2>精确版本，最小披露</h2>
            <p>列表由 Gateway 认证会话派生 principal，再由 Core 返回可用 Definition 的只读投影。</p>
            <dl><dt>WRITE</dt><dd>subscription_autoreply</dd><dt>RUNTIME</dt><dd>disabled</dd><dt>SCOPE</dt><dd>server owned</dd></dl>
            <div class="authority-warning"><strong>目录边界</strong><br>此页面不暴露 owner、tenant、模型、Tool 参数或内部 provenance。订阅创建仍会重新校验 scope。</div>
          </aside>
        </div>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { agentDefinitionCatalogClient, type AgentDefinitionCatalogClient, type AgentDefinitionCatalogItem } from '@/api/agentDefinitions'
import { agentFlags, agentTaskRunTarget } from '@/config/agentFlags'

const props = withDefaults(defineProps<{ client?: AgentDefinitionCatalogClient }>(), {
  client: () => agentDefinitionCatalogClient,
})

type ViewState = 'loading' | 'ready' | 'unavailable'
const viewState = ref<ViewState>('loading')
const definitions = ref<AgentDefinitionCatalogItem[]>([])
const nextCursor = ref('')
const creating = ref(false)
const createError = ref('')
const busy = computed(() => viewState.value === 'loading' || creating.value)
const canCreate = computed(() => props.client.create !== undefined)
const nav = {
  definitions: agentFlags.definitions,
  subscriptions: agentFlags.subscriptions,
  taskRun: agentTaskRunTarget(),
}

onMounted(() => load(true))

async function load(reset: boolean) {
  viewState.value = 'loading'
  if (reset) { definitions.value = []; nextCursor.value = '' }
  try {
    const page = await props.client.list('', 50)
    definitions.value = page.definitions
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    definitions.value = []
    nextCursor.value = ''
    viewState.value = 'unavailable'
  }
}

async function createProfile() {
  if (!props.client.create || creating.value) return
  creating.value = true
  createError.value = ''
  try {
    await props.client.create('subscription_autoreply')
    await load(true)
  } catch {
    createError.value = 'Definition 创建暂不可用。'
    viewState.value = 'unavailable'
  } finally {
    creating.value = false
  }
}

async function loadMore() {
  if (!nextCursor.value || busy.value) return
  viewState.value = 'loading'
  try {
    const page = await props.client.list(nextCursor.value, 50)
    const seen = new Set(definitions.value.map(identity))
    if (page.definitions.some(item => seen.has(identity(item)))) throw new Error('duplicate Agent Definition page')
    definitions.value.push(...page.definitions)
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    definitions.value = []
    nextCursor.value = ''
    viewState.value = 'unavailable'
  }
}

function identity(item: AgentDefinitionCatalogItem) { return `${item.definitionId}:${item.version}` }
function expired(item: AgentDefinitionCatalogItem) { return item.expiresAtUnixMs !== undefined && item.expiresAtUnixMs <= Date.now() }
function agentLabel(agentId: string) { return agentId === 'UAI' ? 'Project Guardian' : `Agent ${agentId}` }
function timestamp(value: number) { return new Date(value).toISOString().replace('T', ' ').replace('.000Z', ' UTC') }
</script>

<style scoped>
.definition-shell{--ink:var(--dp-ink);--muted:var(--dp-ink-soft);--line:var(--dp-line);--panel:var(--dp-surface);--app:var(--dp-canvas);--rail:var(--dp-rail);--green:var(--dp-rail);--green-soft:var(--dp-agent-soft);--amber:var(--dp-warning);--amber-soft:var(--dp-warning-soft);--red:var(--dp-danger);--red-soft:var(--dp-danger-soft);min-height:100vh;background:var(--app);color:var(--ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}.control-rail{background:var(--rail);color:var(--dp-text-inverse);padding:34px 26px;display:flex;flex-direction:column;gap:20px}.brand{font:800 19px var(--dp-font-display);letter-spacing:.12em;display:flex;align-items:center;gap:12px}.brand>span{width:12px;height:12px;border-radius:50%;background:var(--dp-agent)}.rail-kicker,.rail-boundary,.mono,.state-code{font-family:var(--dp-font-data)}.rail-kicker{font-size:10px;color:var(--dp-ink-faint);letter-spacing:.12em;margin-top:8px}.rail-active,.rail-item{border-radius:10px;padding:12px 14px;display:flex;gap:10px;align-items:center;font-size:13px}.rail-active{background:var(--dp-rail-soft);color:var(--dp-text-inverse);font-weight:700}.rail-item{color:var(--dp-ink-faint)}.rail-boundary{margin-top:auto;font-size:9px;line-height:1.9;color:var(--dp-ink-faint)}.definition-main{padding:36px 42px 64px;max-width:1420px;width:100%;margin:auto}.page-header{display:flex;justify-content:space-between;align-items:center;gap:24px}.eyebrow{color:var(--green);font:700 10px var(--dp-font-data);letter-spacing:.1em}.page-header h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:7px 0}.subtitle{color:var(--muted);font-size:13px}.catalog-status{background:var(--panel);border:1px solid var(--line);border-radius:11px;padding:12px 15px;font:10px var(--dp-font-data);color:var(--green)}.boundary-notice{margin:28px 0;display:flex;gap:15px;align-items:center;padding:14px 17px;background:var(--green-soft);border:1px solid var(--green);border-radius:12px;font-size:12px;color:var(--muted)}.boundary-notice strong{color:var(--green);font:700 10px var(--dp-font-data);letter-spacing:.08em}.state-card{min-height:310px;display:flex;flex-direction:column;justify-content:center;align-items:center;text-align:center;border:1px solid var(--line);border-radius:16px;background:var(--panel);padding:32px}.state-card h2{font:800 22px var(--dp-font-display);margin:8px}.state-card p:last-child{color:var(--muted);font-size:13px}.state-code{font-size:10px;color:var(--green);letter-spacing:.12em}.danger{border-color:var(--red)}.danger .state-code,.danger-text{color:var(--red)}.spinner{width:22px;height:22px;border:2px solid var(--line);border-top-color:var(--green);border-radius:50%;animation:spin .8s linear infinite}.text-action,.load-more{border:0;background:transparent;font:700 12px var(--dp-font-data);cursor:pointer}.text-action{margin-top:15px}.list-heading{display:flex;justify-content:space-between;align-items:center;margin:26px 0 13px}.list-heading h2{font:800 12px var(--dp-font-data);letter-spacing:.05em}.list-heading span{color:var(--muted);font:10px var(--dp-font-data)}.content-grid{display:grid;grid-template-columns:minmax(0,1fr) 286px;gap:24px}.definition-list{display:grid;gap:12px}.definition-card{background:var(--panel);border:1px solid var(--line);border-radius:14px;padding:21px}.definition-card.expired{background:var(--dp-surface-muted)}.card-top{display:flex;justify-content:space-between;gap:16px}.card-top h3{font:800 18px var(--dp-font-display);margin:0 0 7px}.mono{font-size:10px;letter-spacing:.04em;color:var(--muted)}.status-pill{height:max-content;border-radius:99px;padding:7px 10px;display:flex;gap:6px;align-items:center;font:700 9px var(--dp-font-data);letter-spacing:.04em}.status-pill i{width:6px;height:6px;border-radius:50%;background:currentColor}.status-active{color:var(--green);background:var(--green-soft)}.status-expired{color:var(--amber);background:var(--amber-soft)}.scope-section{border-top:1px solid var(--line);border-bottom:1px solid var(--line);padding:14px 0;margin:18px 0;display:flex;gap:13px;align-items:flex-start}.scope-label{padding-top:5px;min-width:132px;font:700 9px var(--dp-font-data);letter-spacing:.08em;color:var(--muted)}.scope-chips{display:flex;gap:6px;flex-wrap:wrap}.scope-chip{background:var(--dp-surface-muted);border:1px solid var(--line);border-radius:99px;padding:5px 8px;font:10px var(--dp-font-data);color:var(--ink)}.definition-meta{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:0}.definition-meta div{display:flex;flex-direction:column;gap:5px}.definition-meta dt,.authority-panel dt{font:700 9px var(--dp-font-data);color:var(--muted);letter-spacing:.08em}.definition-meta dd{margin:0;font:11px var(--dp-font-data);color:var(--ink)}.authority-panel{align-self:start;border:1px solid var(--line);border-radius:14px;background:var(--panel);padding:22px}.authority-panel h2{font:800 21px var(--dp-font-display);margin:9px 0}.authority-panel>p:not(.eyebrow){font-size:12px;line-height:1.7;color:var(--muted)}.authority-panel dl{border-top:1px solid var(--line);border-bottom:1px solid var(--line);padding:13px 0;margin:18px 0;display:grid;gap:10px}.authority-panel dl>dt{float:left}.authority-panel dl>dd{margin:0;text-align:right;font:10px var(--dp-font-data);color:var(--green)}.authority-warning{background:var(--amber-soft);color:var(--amber);padding:12px;border-radius:9px;font-size:11px;line-height:1.6}.load-more{color:var(--green);justify-self:start;padding:11px 0}.load-more:disabled{opacity:.5;cursor:default}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:820px){.definition-shell{display:block}.control-rail{display:none}.definition-main{padding:25px 18px 45px}.page-header{align-items:flex-start}.page-header h1{font-size:31px}.catalog-status{padding:9px}.boundary-notice{align-items:flex-start;flex-direction:column;gap:6px}.content-grid{grid-template-columns:1fr}.authority-panel{order:-1}.card-top{align-items:flex-start;flex-direction:column}.scope-section{flex-direction:column;gap:8px}.scope-label{min-width:0}.definition-meta{grid-template-columns:1fr}.state-card{min-height:270px}}
</style>
<style scoped>
a.rail-item,a.rail-active{text-decoration:none;color:inherit}
.header-actions{display:flex;align-items:center;gap:12px}
.create-button{border:0;border-radius:11px;padding:12px 15px;background:var(--dp-accent);color:var(--dp-canvas);cursor:pointer;font:700 12px var(--dp-font-data)}
.create-button:disabled{cursor:wait;opacity:.65}
.form-error{margin:0 0 16px;color:var(--dp-danger);font-size:13px}
</style>
