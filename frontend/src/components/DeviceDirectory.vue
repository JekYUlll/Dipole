<template>
  <section class="device-directory" :data-device-state="state" :aria-busy="state === 'loading'">
    <aside class="device-rail" aria-label="Primary navigation">
      <img class="brand-logo" :src="dipoleLogo" alt="Dipole IM" />
      <span>SECURITY / SESSIONS</span>
      <nav>
        <RouterLink :to="{ name: 'chat' }">会话</RouterLink>
        <RouterLink :to="{ name: 'contacts' }">联系人</RouterLink>
        <RouterLink :to="{ name: 'groups' }">群组</RouterLink>
        <RouterLink class="active" :to="{ name: 'devices' }">设备</RouterLink>
      </nav>
      <p>OWNER SCOPED<br />SESSION CONTROL</p>
    </aside>
    <main>
      <div class="mobile-brandbar"><img :src="dipoleLogo" alt="Dipole IM" /><span>设备会话</span><RouterLink :to="{ name: 'chat' }">返回会话</RouterLink></div>
      <header><div><p class="eyebrow">DEVICE SESSIONS</p><h1>设备会话</h1><p class="subtitle">查看并撤销当前账户的在线连接。</p></div><span class="mode">仅限本人</span></header>
      <p class="boundary" role="note"><strong>安全边界</strong>会话由服务端按当前认证账户返回；设备 ID、节点和地址只用于识别连接，不代表可信设备认证。</p>
      <section v-if="state === 'loading'" class="state-card" role="status"><i class="spinner" /><h2>正在读取设备</h2><p>仅加载当前账户的服务端会话投影。</p></section>
      <section v-else-if="state === 'unavailable'" class="state-card unavailable" role="alert"><p class="eyebrow">UNAVAILABLE</p><h2>设备会话暂时不可用</h2><p>已清空旧会话，避免将过期连接当作当前状态。</p><button data-device-retry @click="load">重新确认</button></section>
      <section v-else-if="sessions.length === 0" class="state-card" role="status"><p class="eyebrow">EMPTY</p><h2>当前没有在线设备</h2><p>重新登录后，新的会话会出现在这里。</p></section>
      <template v-else>
        <div class="list-heading"><h2>在线连接 {{ String(sessions.length).padStart(2, '0') }}</h2><button class="logout-all" :disabled="busy !== null" @click="logoutAll">全部下线</button></div>
        <div class="session-list">
          <article v-for="session in sessions" :key="session.connection_id" class="session-card">
            <div class="device-icon" aria-hidden="true">{{ deviceInitial(session) }}</div>
            <div class="identity"><h3>{{ session.device }}</h3><p>{{ session.user_agent || '未提供浏览器信息' }}</p><small>{{ session.remote_addr || '地址未提供' }} · 节点 {{ session.node_id }}</small></div>
            <div class="session-meta"><span class="status">在线</span><time :datetime="session.last_seen_at">{{ formatTime(session.last_seen_at) }}</time><button :disabled="busy !== null" @click="logout(session.connection_id)">{{ busy === session.connection_id ? '下线中' : '下线' }}</button></div>
          </article>
        </div>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { deviceSessionClient, type DeviceSession, type DeviceSessionClient } from '@/api/devices'
import dipoleLogo from '../../../docs/images/dipole-v3-im-mark-traced.svg'

const props = withDefaults(defineProps<{ client?: DeviceSessionClient }>(), { client: () => deviceSessionClient })
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const sessions = ref<DeviceSession[]>([])
const busy = ref<string | 'all' | null>(null)
onMounted(load)

async function load() {
  state.value = 'loading'; sessions.value = []
  try { sessions.value = await props.client.list(); state.value = 'ready' } catch { state.value = 'unavailable' }
}
async function logout(connectionID: string) {
  busy.value = connectionID
  try { await props.client.logout(connectionID); sessions.value = sessions.value.filter(item => item.connection_id !== connectionID) } catch { state.value = 'unavailable'; sessions.value = [] } finally { busy.value = null }
}
async function logoutAll() {
  busy.value = 'all'
  try { await props.client.logoutAll(); sessions.value = [] } catch { state.value = 'unavailable'; sessions.value = [] } finally { busy.value = null }
}
function deviceInitial(session: DeviceSession) { return session.device.trim().slice(0, 1).toUpperCase() || 'D' }
function formatTime(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? '时间未知' : new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(date) }
</script>

<style scoped>
.device-directory{min-height:100vh;background:var(--dp-canvas);color:var(--dp-ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}
.device-rail{background:var(--dp-rail);color:var(--dp-text-inverse);padding:34px 26px;display:flex;flex-direction:column;gap:12px}.device-rail>span,.device-rail p{font:10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-ink-faint)}.device-rail nav{display:grid;gap:7px;margin-top:24px}.device-rail nav a{padding:11px 13px;border-radius:var(--dp-radius-sm);font-size:13px;color:var(--dp-ink-faint);text-decoration:none}.device-rail nav .active{background:var(--dp-rail-soft);color:var(--dp-text-inverse)}.device-rail p{margin-top:auto;line-height:1.8}
main{max-width:1280px;width:100%;margin:auto;padding:42px 54px 68px;box-sizing:border-box}header{display:flex;justify-content:space-between;align-items:center;gap:20px}.eyebrow{font:700 10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-accent-strong);margin:0}.subtitle{color:var(--dp-ink-soft);font-size:14px;margin:6px 0 0}h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:7px 0}.mode{border:1px solid var(--dp-line);background:var(--dp-surface);padding:10px 13px;border-radius:var(--dp-radius-sm);color:var(--dp-accent-strong);font:700 11px var(--dp-font-data)}.boundary{margin:28px 0;padding:14px 17px;border:1px solid var(--dp-accent);background:var(--dp-accent-soft);border-radius:12px;color:var(--dp-ink-soft);font-size:13px}.boundary strong{color:var(--dp-accent-strong);font:700 10px var(--dp-font-data);letter-spacing:.08em;margin-right:14px}.state-card{min-height:360px;background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:16px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:32px}.state-card h2{font:800 22px var(--dp-font-display);margin:12px}.state-card p:not(.eyebrow){color:var(--dp-ink-soft);font-size:13px}.unavailable{border-color:var(--dp-danger)}button{border:0;border-radius:9px;background:var(--dp-rail);color:var(--dp-text-inverse);padding:10px 15px;font:700 12px var(--dp-font-body);cursor:pointer}button:disabled{cursor:not-allowed;opacity:.5}.spinner{width:23px;height:23px;border:2px solid var(--dp-line);border-top-color:var(--dp-accent);border-radius:50%;animation:spin .8s linear infinite}.list-heading{display:flex;justify-content:space-between;align-items:center;margin:26px 0 13px}.list-heading h2{font:800 12px var(--dp-font-data);letter-spacing:.05em}.logout-all{background:transparent;color:var(--dp-danger);border:1px solid var(--dp-danger);padding:8px 12px}.session-list{display:grid;gap:12px}.session-card{background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:14px;display:flex;gap:15px;align-items:center;padding:20px}.device-icon{display:grid;place-items:center;flex:0 0 46px;width:46px;height:46px;border-radius:12px;background:var(--dp-rail);color:var(--dp-text-inverse);font:800 18px var(--dp-font-display)}.identity{min-width:0;flex:1}.identity h3{font:800 18px var(--dp-font-display);margin:0 0 5px}.identity p{color:var(--dp-ink-soft);font-size:13px;margin:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.identity small{display:block;color:var(--dp-ink-faint);font-size:11px;margin-top:6px}.session-meta{display:flex;align-items:center;gap:12px}.session-meta time{color:var(--dp-ink-faint);font:11px var(--dp-font-data);white-space:nowrap}.status{border-radius:99px;background:var(--dp-accent-soft);color:var(--dp-accent-strong);padding:7px 10px;font:700 10px var(--dp-font-data)}
@keyframes spin{to{transform:rotate(360deg)}}.brand-logo{display:block;width:116px;height:86px;object-fit:contain;object-position:center;margin:-8px 0}.mobile-brandbar{display:none}
@media(max-width:760px){.device-directory{display:block}.device-rail{display:none}main{padding:28px 18px 48px}header{align-items:flex-start}h1{font-size:31px}.mode{padding:9px}.boundary{line-height:1.7}.boundary strong{display:block;margin-bottom:5px}.session-card{padding:16px;align-items:flex-start}.session-meta{margin-left:auto;display:grid;justify-items:end;gap:7px}.session-meta time{font-size:10px}.identity h3{font-size:16px}.identity p{white-space:normal;line-height:1.5}.mobile-brandbar{display:flex;align-items:center;gap:10px;border-bottom:1px solid var(--dp-line);padding:0 0 17px;margin-bottom:28px}.mobile-brandbar img{width:48px;height:36px;object-fit:contain}.mobile-brandbar span{font:800 15px var(--dp-font-display);flex:1}.mobile-brandbar a{color:var(--dp-accent-strong);font-size:12px;font-weight:700;text-decoration:none}}
@media(prefers-reduced-motion:reduce){.spinner{animation:none}}
</style>
