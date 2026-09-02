<template>
  <section class="device-security" :data-device-state="state" :aria-busy="state === 'loading'">
    <aside class="device-rail" aria-label="Primary navigation"><strong>DIPOLE</strong><span>DEVICE SECURITY</span><nav><span>会话</span><span>联系人</span><span>群组</span><span>文件</span><b>设备</b></nav><p>AUTHENTICATED<br>PRIVACY-SAFE<br>SESSION CONTROL</p></aside>
    <main>
      <header><div><p class="eyebrow">DEVICE SECURITY</p><h1>设备会话</h1><p class="subtitle">查看当前已登录设备，并保留当前会话地登出其他设备。</p></div><button v-if="otherSessions.length" class="logout-others" data-device-logout-others data-testid="logout-others" @click="confirmation = { kind: 'others' }">登出其他设备</button></header>
      <p class="boundary" role="note"><strong>隐私边界</strong>此页面只显示设备名称、粗粒度设备类型和相对活动时间，不显示 IP、节点、连接 ID 或原始浏览器信息。</p>
      <section v-if="state === 'loading'" class="state-card" role="status"><i class="spinner" /><h2>正在读取设备会话</h2><p>正在确认当前认证账户的在线会话。</p></section>
      <section v-else-if="state === 'unavailable'" class="state-card unavailable" role="alert"><p class="eyebrow">UNAVAILABLE</p><h2>设备会话暂时不可用</h2><p>旧会话已清空，避免将过期设备状态误认为当前授权结果。</p><button data-device-retry @click="load">重新确认</button></section>
      <section v-else-if="sessions.length === 0" class="state-card" role="status"><p class="eyebrow">EMPTY</p><h2>当前没有在线设备</h2><p>设备会话会在新的实时连接建立后显示。</p></section>
      <template v-else>
        <section class="summary" aria-label="Session summary"><article><b>{{ currentSession ? '1' : '0' }}</b><span>当前会话</span></article><article><b>{{ otherSessions.length }}</b><span>其他设备</span></article><article><b>粗粒度</b><span>隐私披露</span></article></section>
        <section class="session-list"><h2>已登录设备</h2><article v-for="session in sessions" :key="session.connection_id" class="session-card" :data-current="isCurrent(session)"><div class="device-icon" aria-hidden="true">{{ deviceGlyph(session) }}</div><div class="identity"><h3>{{ label(session) }}</h3><p>{{ detail(session) }} · {{ relativeTime(session.last_seen_at) }}</p></div><span class="trust" :class="{ current: isCurrent(session) }">{{ isCurrent(session) ? '当前会话' : '已确认' }}</span><button v-if="!isCurrent(session)" class="logout-one" :disabled="acting" @click="confirmation = { kind: 'one', session }">登出</button></article></section>
        <section v-if="confirmation" class="confirmation" role="alert"><div><p class="eyebrow">需要确认</p><h2>{{ confirmation.kind === 'others' ? '登出所有其他设备？' : `登出 ${label(confirmation.session)}？` }}</h2><p>{{ confirmation.kind === 'others' ? '当前设备会保持登录，其他列出的会话需要再次认证。' : '这会结束所选设备的访问权限。' }}</p></div><div class="confirmation-actions"><button class="cancel" :disabled="acting" @click="confirmation = undefined">取消</button><button class="confirm" :disabled="acting" data-device-confirm-logout data-testid="confirm-logout" @click="confirmLogout">{{ acting ? '处理中...' : '确认登出' }}</button></div></section>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getDeviceID } from '@/device'
import { deviceSessionClient, type DeviceSession, type DeviceSessionClient } from '@/api/devices'

const props = withDefaults(defineProps<{ client?: DeviceSessionClient; currentDeviceID?: string }>(), { client: () => deviceSessionClient, currentDeviceID: () => getDeviceID() })
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const sessions = ref<DeviceSession[]>([])
const acting = ref(false)
const confirmation = ref<{ kind: 'others' } | { kind: 'one'; session: DeviceSession }>()
const currentSession = computed(() => sessions.value.find(isCurrent))
const otherSessions = computed(() => sessions.value.filter(session => !isCurrent(session)))

onMounted(load)

async function load() {
  state.value = 'loading'; sessions.value = []; confirmation.value = undefined
  try { sessions.value = await props.client.list(); state.value = 'ready' } catch { state.value = 'unavailable' }
}

async function confirmLogout() {
  if (!confirmation.value || acting.value) return
  acting.value = true
  try {
    if (confirmation.value.kind === 'others') await props.client.logoutOthers()
    else await props.client.logout(confirmation.value.session.connection_id)
    await load()
  } catch { state.value = 'unavailable'; sessions.value = [] }
  finally { acting.value = false }
}

function isCurrent(session: DeviceSession) { return Boolean(session.device_id && session.device_id === props.currentDeviceID) }
function label(session: DeviceSession) { return session.device === 'web' ? '浏览器会话' : session.device === 'mobile' ? '移动设备' : session.device === 'desktop' ? '桌面设备' : session.device }
function detail(session: DeviceSession) { return session.device === 'web' ? '浏览器' : session.device === 'mobile' ? '移动端' : '桌面端' }
function deviceGlyph(session: DeviceSession) { return session.device === 'mobile' ? 'M' : session.device === 'web' ? 'W' : 'D' }
function relativeTime(value: string) { const minutes = Math.max(0, Math.floor((Date.now() - Date.parse(value)) / 60000)); return minutes < 1 ? '刚刚活跃' : minutes < 60 ? `${minutes} 分钟前活跃` : `${Math.floor(minutes / 60)} 小时前活跃` }
</script>

<style scoped>
.device-security{min-height:100vh;background:var(--dp-canvas);color:var(--dp-ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}.device-rail{background:var(--dp-rail);color:var(--dp-text-inverse);padding:34px 26px;display:flex;flex-direction:column;gap:12px}.device-rail strong{font:800 20px var(--dp-font-display);letter-spacing:.11em}.device-rail>span,.device-rail p{font:10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-ink-faint)}.device-rail nav{display:grid;gap:7px;margin-top:24px}.device-rail nav>*{padding:11px 13px;border-radius:10px;font-size:13px;color:#b7c7c0}.device-rail nav b{background:var(--dp-rail-soft);color:var(--dp-text-inverse)}.device-rail p{margin-top:auto;line-height:1.8}main{max-width:1280px;width:100%;margin:auto;padding:42px 54px 68px;box-sizing:border-box}header{display:flex;justify-content:space-between;align-items:center;gap:20px}.eyebrow{font:700 10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-accent-strong);margin:0}.subtitle{color:var(--dp-ink-soft);font-size:14px;margin:6px 0 0}h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:7px 0}.logout-others,.confirm{border:0;border-radius:9px;background:var(--dp-warning);color:var(--dp-text-inverse);padding:11px 15px;font:700 13px var(--dp-font-body);cursor:pointer}.boundary{margin:28px 0;padding:14px 17px;border:1px solid var(--dp-accent);background:var(--dp-accent-soft);border-radius:12px;color:var(--dp-ink-soft);font-size:13px}.boundary strong{color:var(--dp-accent-strong);font:700 10px var(--dp-font-data);letter-spacing:.08em;margin-right:14px}.state-card{min-height:360px;background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:16px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:32px}.state-card h2{font:800 22px var(--dp-font-display);margin:12px}.state-card p:not(.eyebrow){color:var(--dp-ink-soft);font-size:13px}.unavailable{border-color:var(--dp-danger)}button{cursor:pointer}.summary{display:grid;grid-template-columns:repeat(3,1fr);gap:14px;margin-bottom:28px}.summary article{background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:14px;padding:20px}.summary b{display:block;font:800 28px var(--dp-font-display)}.summary span{display:block;color:var(--dp-ink-soft);font-size:13px;margin-top:8px}.session-list h2{font:800 14px var(--dp-font-data);letter-spacing:.05em;margin-bottom:13px}.session-list{display:grid;gap:12px}.session-card{display:flex;gap:15px;align-items:center;background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:14px;padding:18px}.device-icon{display:grid;place-items:center;flex:0 0 46px;width:46px;height:46px;border-radius:12px;background:var(--dp-accent-soft);color:var(--dp-accent-strong);font:800 13px var(--dp-font-data)}.identity{min-width:0;flex:1}.identity h3{font:800 18px var(--dp-font-display);margin:0 0 5px}.identity p{color:var(--dp-ink-soft);font-size:13px;margin:0}.trust{border:1px solid var(--dp-line);padding:7px 10px;border-radius:99px;color:var(--dp-ink-soft);font:700 11px var(--dp-font-data);white-space:nowrap}.trust.current{border-color:#b8e3cb;background:var(--dp-accent-soft);color:var(--dp-accent-strong)}.logout-one,.cancel{border:1px solid #f2d7a7;background:#fff7ea;color:var(--dp-warning);border-radius:8px;padding:9px 12px;font:700 12px var(--dp-font-body)}.confirmation{margin-top:20px;padding:18px 20px;border:1px solid #f0c879;background:#fff7ea;border-radius:14px;display:flex;gap:18px;align-items:center}.confirmation h2{font:800 18px var(--dp-font-display);margin:6px 0}.confirmation p:not(.eyebrow){margin:0;color:var(--dp-ink-soft);font-size:13px}.confirmation-actions{margin-left:auto;display:flex;gap:8px;white-space:nowrap}.confirm{background:var(--dp-rail)}.spinner{width:23px;height:23px;border:2px solid var(--dp-line);border-top-color:var(--dp-accent);border-radius:50%;animation:spin .8s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:760px){.device-security{display:block}.device-rail{display:none}main{padding:28px 18px 48px}header{align-items:flex-start;flex-direction:column}h1{font-size:31px}.boundary{line-height:1.7}.boundary strong{display:block;margin-bottom:5px}.summary{gap:9px}.summary article{padding:14px}.summary b{font-size:20px}.summary span{font-size:11px}.session-card{align-items:flex-start;flex-wrap:wrap}.identity{min-width:calc(100% - 61px)}.trust{margin-left:61px}.logout-one{margin-left:auto}.confirmation{align-items:stretch;flex-direction:column}.confirmation-actions{margin-left:0}.confirmation-actions>*{flex:1}}@media(prefers-reduced-motion:reduce){.spinner{animation:none}}
</style>
