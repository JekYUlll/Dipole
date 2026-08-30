<template>
  <main class="settings-page" aria-labelledby="settings-title">
    <header class="settings-header">
      <div>
        <p class="eyebrow">ACCOUNT / CLIENT</p>
        <h1 id="settings-title">设置</h1>
        <p>管理个人资料、已登录设备和本地同步边界。</p>
      </div>
      <RouterLink class="back-link" :to="{ name: 'chat' }">返回消息</RouterLink>
    </header>

    <p v-if="loadError" class="notice notice-error" role="alert">{{ loadError }} <button type="button" @click="load">重试</button></p>

    <section class="settings-card" aria-labelledby="profile-title">
      <div class="section-heading">
        <div>
          <p class="eyebrow">PROFILE</p>
          <h2 id="profile-title">个人资料</h2>
        </div>
        <span v-if="auth.currentUser" class="identity">{{ auth.currentUser.telephone || auth.currentUser.email || auth.currentUser.uuid }}</span>
      </div>
      <div class="profile-grid">
        <div class="avatar" aria-hidden="true">{{ initials }}</div>
        <label class="signature-field">
          <span>个性签名</span>
          <textarea v-model="signature" maxlength="255" :disabled="loading || saving" placeholder="写下想让协作者了解的一句话" />
          <small>{{ signature.length }}/255</small>
        </label>
      </div>
      <div class="actions">
        <span v-if="profileSaved" class="notice notice-success" role="status">资料已保存</span>
        <button class="primary" type="button" :disabled="loading || saving || !auth.currentUser" @click="saveProfile">
          {{ saving ? '保存中...' : '保存资料' }}
        </button>
      </div>
    </section>

    <section class="settings-card" aria-labelledby="devices-title">
      <div class="section-heading">
        <div>
          <p class="eyebrow">SECURITY</p>
          <h2 id="devices-title">已登录设备</h2>
        </div>
        <button class="text-button" type="button" :disabled="loading" @click="refreshDevices">刷新</button>
      </div>
      <p class="section-copy">设备会话由服务端维护。移除设备会撤销对应会话并向该连接发送下线通知。</p>
      <div v-if="loading" class="empty-state">正在读取设备会话...</div>
      <div v-else-if="chat.devices.length === 0" class="empty-state">当前没有可展示的设备会话。</div>
      <ul v-else class="device-list">
        <li v-for="device in chat.devices" :key="device.connection_id" class="device-row">
          <div>
            <strong>{{ device.device || '未知设备' }}</strong>
            <p>{{ device.ip || 'IP 未提供' }} · {{ formatDate(device.connected_at) }}</p>
          </div>
          <button class="danger-outline" type="button" :disabled="revoking === device.connection_id" @click="logoutDevice(device.connection_id)">
            {{ revoking === device.connection_id ? '处理中...' : '移除此设备' }}
          </button>
        </li>
      </ul>
      <div class="danger-zone">
        <div>
          <strong>退出所有设备</strong>
          <p>会撤销当前账户的全部会话，包括本设备。</p>
        </div>
        <button class="danger" type="button" :disabled="loggingOutAll" @click="logoutAll">
          {{ loggingOutAll ? '退出中...' : '退出所有设备' }}
        </button>
      </div>
    </section>

    <section class="settings-card" aria-labelledby="sync-title">
      <div class="section-heading">
        <div>
          <p class="eyebrow">LOCAL SYNC</p>
          <h2 id="sync-title">客户端同步</h2>
        </div>
        <span class="sync-state">{{ syncLabel }}</span>
      </div>
      <p class="section-copy">本地消息按登录用户隔离。退出登录、身份失效和 WebSocket 下线会清理当前账号的本地同步数据。</p>
      <dl class="sync-details">
        <div><dt>安全游标</dt><dd>{{ chat.safeSyncSeq || '尚未建立' }}</dd></div>
        <div><dt>同步状态</dt><dd>{{ syncLabel }}</dd></div>
      </dl>
    </section>

    <section class="settings-card logout-card" aria-labelledby="logout-title">
      <div>
        <p class="eyebrow">SESSION</p>
        <h2 id="logout-title">退出当前账户</h2>
        <p>退出后会撤销当前会话并清理本机保存的账户数据。</p>
      </div>
      <button class="danger" type="button" @click="logout">退出登录</button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import api from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'

const auth = useAuthStore()
const chat = useChatStore()
const router = useRouter()
const signature = ref('')
const loading = ref(true)
const saving = ref(false)
const loggingOutAll = ref(false)
const revoking = ref('')
const loadError = ref('')
const profileSaved = ref(false)

const initials = computed(() => (auth.currentUser?.nickname || '?').slice(0, 1).toUpperCase())
const syncLabel = computed(() => ({
  idle: '尚未同步', restoring: '正在恢复', current: '已同步', error: '同步异常', storage_full: '本地空间不足',
}[chat.syncStatus] || '未知状态'))

const load = async () => {
  loading.value = true
  loadError.value = ''
  try {
    await Promise.all([auth.fetchMe(), chat.fetchDevices()])
    signature.value = auth.currentUser?.signature || ''
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '设置加载失败'
  } finally {
    loading.value = false
  }
}

const saveProfile = async () => {
  if (!auth.currentUser) return
  saving.value = true
  profileSaved.value = false
  try {
    await api.patch(`/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/profile`, { signature: signature.value.trim() })
    await auth.fetchMe()
    signature.value = auth.currentUser?.signature || ''
    profileSaved.value = true
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '资料保存失败'
  } finally {
    saving.value = false
  }
}

const refreshDevices = async () => {
  try { await chat.fetchDevices() } catch (error) { loadError.value = error instanceof Error ? error.message : '设备刷新失败' }
}

const logoutDevice = async (connectionID: string) => {
  if (!connectionID || !window.confirm('确认移除这个设备吗？该设备会立即下线。')) return
  revoking.value = connectionID
  try {
    await api.post(`/api/v1/users/me/devices/${encodeURIComponent(connectionID)}/logout`)
    await chat.fetchDevices()
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '设备下线失败'
  } finally {
    revoking.value = ''
  }
}

const logoutAll = async () => {
  if (!window.confirm('确认退出所有设备吗？本设备也会退出。')) return
  loggingOutAll.value = true
  try {
    await api.post('/api/v1/users/me/devices/logout-all')
    await auth.terminateSession(true)
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '退出所有设备失败'
  } finally {
    loggingOutAll.value = false
  }
}

const logout = async () => {
  await auth.logout()
  await router.push({ name: 'login' })
}

const formatDate = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '连接时间未知'
onMounted(load)
</script>

<style scoped>
.settings-page { min-height: 100vh; padding: 48px max(24px, calc((100vw - 1040px) / 2)); background: var(--dp-canvas); color: var(--dp-ink); font-family: var(--dp-font-body); }
.settings-header, .section-heading, .actions, .danger-zone, .logout-card { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.settings-header { margin-bottom: 28px; }.settings-header h1, h2, p { margin: 0; }.settings-header h1 { font-family: var(--dp-font-display); font-size: 36px; letter-spacing: -.04em; }.settings-header > div > p:last-child, .section-copy, .danger-zone p, .logout-card p { color: var(--dp-ink-muted); margin-top: 8px; }
.eyebrow { color: var(--dp-accent); font-size: 11px; font-weight: 800; letter-spacing: .12em; }.back-link, .text-button { color: var(--dp-accent); font-weight: 700; text-decoration: none; background: transparent; border: 0; cursor: pointer; }
.settings-card { margin-top: 16px; padding: 28px; border: 1px solid var(--dp-border); border-radius: var(--dp-radius-lg); background: var(--dp-surface); box-shadow: var(--dp-shadow-sm); }.settings-card h2 { font-size: 20px; }.identity, .sync-state { color: var(--dp-ink-muted); font-size: 13px; }
.profile-grid { display: flex; gap: 20px; align-items: flex-start; margin-top: 24px; }.avatar { display: grid; place-items: center; width: 58px; height: 58px; border-radius: 18px; background: var(--dp-accent); color: white; font-size: 24px; font-weight: 800; }.signature-field { flex: 1; display: grid; gap: 8px; font-weight: 700; }.signature-field textarea { min-height: 86px; padding: 12px; border: 1px solid var(--dp-border); border-radius: var(--dp-radius-md); resize: vertical; font: inherit; }.signature-field small { color: var(--dp-ink-muted); text-align: right; }.actions { margin-top: 18px; justify-content: flex-end; }
button { font: inherit; } .primary, .danger, .danger-outline { border-radius: var(--dp-radius-md); padding: 10px 14px; font-weight: 750; cursor: pointer; }.primary { border: 1px solid var(--dp-accent); background: var(--dp-accent); color: white; }.danger { border: 1px solid #c4474e; background: #c4474e; color: white; }.danger-outline { border: 1px solid #d9979b; background: transparent; color: #9e343b; }.primary:disabled, .danger:disabled, .danger-outline:disabled, .text-button:disabled { cursor: not-allowed; opacity: .55; }
.device-list { list-style: none; margin: 18px 0 0; padding: 0; }.device-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 16px 0; border-top: 1px solid var(--dp-border); }.device-row p { color: var(--dp-ink-muted); font-size: 13px; margin-top: 5px; }.empty-state { margin-top: 18px; padding: 20px; color: var(--dp-ink-muted); background: var(--dp-surface-muted); border-radius: var(--dp-radius-md); }.danger-zone { margin-top: 20px; padding-top: 20px; border-top: 1px solid var(--dp-border); }.sync-details { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin: 20px 0 0; }.sync-details div { padding: 16px; border-radius: var(--dp-radius-md); background: var(--dp-surface-muted); }.sync-details dt { color: var(--dp-ink-muted); font-size: 12px; }.sync-details dd { margin: 5px 0 0; font-weight: 750; }.logout-card { margin-bottom: 40px; }.notice { display: inline-flex; gap: 8px; align-items: center; margin: 0 0 16px; padding: 10px 12px; border-radius: var(--dp-radius-md); }.notice button { border: 0; background: transparent; color: inherit; font-weight: 800; text-decoration: underline; cursor: pointer; }.notice-error { background: #fff1f1; color: #9e343b; }.notice-success { background: #edf9f0; color: #25753d; }
@media (max-width: 640px) { .settings-page { padding: 28px 16px; }.settings-header, .section-heading, .danger-zone, .logout-card { align-items: flex-start; flex-direction: column; }.settings-header h1 { font-size: 30px; }.settings-card { padding: 20px; }.profile-grid { gap: 14px; }.device-row { align-items: flex-start; flex-direction: column; }.sync-details { grid-template-columns: 1fr; }.actions { align-items: stretch; flex-direction: column; }.primary { width: 100%; } }
</style>
