<template>
  <main class="settings-page" aria-labelledby="settings-title">
    <header class="settings-header">
      <div>
        <p class="eyebrow">ACCOUNT / CLIENT</p>
        <h1 id="settings-title">设置</h1>
        <p>管理个人资料、本地同步状态和账户会话。</p>
      </div>
      <RouterLink class="back-link" :to="{ name: 'chat' }">返回消息</RouterLink>
    </header>

    <p v-if="errorMessage" class="notice error" role="alert">{{ errorMessage }} <button type="button" @click="load">重试</button></p>

    <section class="settings-card" aria-labelledby="profile-title">
      <div class="card-heading">
        <div><p class="eyebrow">PROFILE</p><h2 id="profile-title">个人资料</h2></div>
        <span class="identity">{{ accountIdentity }}</span>
      </div>
      <label class="signature-field">
        <span>个性签名</span>
        <textarea v-model="signature" maxlength="255" :disabled="loading || saving" placeholder="写下想让协作者了解的一句话" />
        <small>{{ signature.length }}/255</small>
      </label>
      <div class="actions">
        <span v-if="saved" class="notice success" role="status">资料已保存</span>
        <button class="primary" type="button" :disabled="loading || saving || !auth.currentUser" @click="saveProfile">{{ saving ? '保存中...' : '保存资料' }}</button>
      </div>
    </section>

    <section v-if="agentLinks.length" class="settings-card" aria-labelledby="agent-title">
      <div class="card-heading">
        <div><p class="eyebrow">AGENT CONTROL</p><h2 id="agent-title">Agent 控制</h2></div>
      </div>
      <p class="card-copy">打开当前账号已启用的 Agent 页面。任务审批和补充信息仍要从具体任务的时间线进入。</p>
      <ul class="agent-links">
        <li v-for="link in agentLinks" :key="link.id">
          <RouterLink class="secondary-link" :to="link.to">{{ link.label }}</RouterLink>
        </li>
      </ul>
    </section>

    <section class="settings-card" aria-labelledby="device-title">
      <div class="card-heading">
        <div><p class="eyebrow">SECURITY</p><h2 id="device-title">设备会话</h2></div>
        <RouterLink class="secondary-link" :to="{ name: 'devices' }">打开设备安全</RouterLink>
      </div>
      <p class="card-copy">设备管理使用独立的隐私安全页面，仅披露设备名称、粗粒度类型和活动时间。该页面不显示 IP、节点或连接标识。</p>
    </section>

    <section class="settings-card" aria-labelledby="sync-title">
      <div class="card-heading">
        <div><p class="eyebrow">LOCAL SYNC</p><h2 id="sync-title">客户端同步</h2></div>
        <span class="sync-state">{{ syncLabel }}</span>
      </div>
      <p class="card-copy">本地消息按登录用户隔离。退出登录、身份失效和 WebSocket 下线会清理当前账号的同步数据。</p>
      <dl class="sync-details"><div><dt>安全游标</dt><dd>{{ chat.safeSyncSeq || '尚未建立' }}</dd></div><div><dt>同步状态</dt><dd>{{ syncLabel }}</dd></div></dl>
    </section>

    <section class="settings-card logout-card" aria-labelledby="logout-title">
      <div><p class="eyebrow">SESSION</p><h2 id="logout-title">退出当前账户</h2><p class="card-copy">退出后会撤销当前会话并清理本机保存的账户数据。</p></div>
      <button class="danger" type="button" @click="logout">退出登录</button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import api from '@/api'
import { agentSettingsLinks } from '@/config/agentFlags'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'

const auth = useAuthStore()
const chat = useChatStore()
const router = useRouter()
const signature = ref('')
const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const errorMessage = ref('')
const accountIdentity = computed(() => auth.currentUser?.telephone || auth.currentUser?.email || auth.currentUser?.uuid || '未登录')
const syncLabel = computed(() => ({ idle: '尚未同步', restoring: '正在恢复', current: '已同步', error: '同步异常', storage_full: '本地空间不足' }[chat.syncStatus] || '未知状态'))
const agentLinks = agentSettingsLinks()

async function load() {
  loading.value = true; errorMessage.value = ''
  try { await auth.fetchMe(); signature.value = auth.currentUser?.signature || '' }
  catch (error) { errorMessage.value = error instanceof Error ? error.message : '设置加载失败' }
  finally { loading.value = false }
}

async function saveProfile() {
  if (!auth.currentUser) return
  saving.value = true; saved.value = false
  try {
    await api.patch(`/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/profile`, { signature: signature.value.trim() })
    await auth.fetchMe(); signature.value = auth.currentUser?.signature || ''; saved.value = true
  } catch (error) { errorMessage.value = error instanceof Error ? error.message : '资料保存失败' }
  finally { saving.value = false }
}

async function logout() { await auth.logout(); await router.push({ name: 'login' }) }
onMounted(load)
</script>

<style scoped>
.settings-page{min-height:100vh;background:var(--dp-canvas);color:var(--dp-ink);font-family:var(--dp-font-body);padding:48px max(24px,calc((100vw - 960px)/2))}.settings-header,.card-heading,.actions,.logout-card{display:flex;align-items:center;justify-content:space-between;gap:20px}.settings-header{margin-bottom:28px}.settings-header h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:6px 0}.settings-header>div>p:last-child,.card-copy{color:var(--dp-ink-soft);margin:0;line-height:1.7}.eyebrow{color:var(--dp-accent-strong);font:700 10px var(--dp-font-data);letter-spacing:.12em;margin:0}.back-link,.secondary-link{color:var(--dp-accent-strong);font-weight:700;text-decoration:none}.settings-card{background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:var(--dp-radius-md);box-shadow:0 12px 30px color-mix(in srgb, var(--dp-rail) 5%, transparent);margin-top:16px;padding:28px}.settings-card h2{font:800 21px var(--dp-font-display);margin:5px 0 0}.identity,.sync-state{color:var(--dp-ink-soft);font:600 12px var(--dp-font-data)}.agent-links{display:flex;flex-wrap:wrap;gap:12px 20px;list-style:none;margin:18px 0 0;padding:0}.signature-field{display:grid;gap:8px;font-weight:700;margin-top:22px}.signature-field textarea{border:1px solid var(--dp-line);border-radius:var(--dp-radius-sm);font:inherit;min-height:92px;padding:12px;resize:vertical}.signature-field small{color:var(--dp-ink-faint);text-align:right}.actions{justify-content:flex-end;margin-top:16px}.primary,.danger{border:0;border-radius:var(--dp-radius-sm);color:var(--dp-text-inverse);cursor:pointer;font:700 13px var(--dp-font-body);padding:11px 16px}.primary{background:var(--dp-accent-strong)}.danger{background:var(--dp-danger)}button:disabled{cursor:not-allowed;opacity:.6}.sync-details{display:grid;gap:12px;grid-template-columns:repeat(2,minmax(0,1fr));margin:20px 0 0}.sync-details div{background:var(--dp-surface-muted);border-radius:var(--dp-radius-sm);padding:15px}.sync-details dt{color:var(--dp-ink-soft);font-size:12px}.sync-details dd{font:700 15px var(--dp-font-data);margin:6px 0 0}.notice{border-radius:var(--dp-radius-sm);display:inline-flex;gap:8px;margin:0 0 16px;padding:10px 12px}.notice button{background:transparent;border:0;color:inherit;cursor:pointer;font-weight:700;text-decoration:underline}.error{background:var(--dp-danger-soft);color:var(--dp-danger)}.success{background:var(--dp-success-soft);color:var(--dp-success);margin:0}.logout-card{margin-bottom:40px}@media(max-width:640px){.settings-page{padding:28px 16px}.settings-header,.card-heading,.logout-card{align-items:flex-start;flex-direction:column}.settings-card{padding:20px}.sync-details{grid-template-columns:1fr}.actions{align-items:stretch;flex-direction:column}.primary,.danger{width:100%}}
</style>
