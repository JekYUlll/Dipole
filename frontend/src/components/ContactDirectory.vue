<template>
  <section class="contact-directory" :data-contact-state="state" :aria-busy="state === 'loading'">
    <aside class="contact-rail" aria-label="Primary navigation">
      <img class="brand-logo" :src="dipoleLogo" alt="Dipole IM" />
      <span>PEOPLE / TRUST</span>
      <nav>
        <RouterLink class="active" :to="{ name: 'contacts' }">联系人</RouterLink>
        <RouterLink :to="{ name: 'chat' }">会话</RouterLink>
        <span>群组</span><span>文件</span><span>设备</span>
      </nav>
      <p>READ ONLY<br>AUTHENTICATED SCOPE</p>
    </aside>

    <main>
      <div class="mobile-brandbar"><img :src="dipoleLogo" alt="Dipole IM" /><span>联系人</span><RouterLink :to="{ name: 'chat' }">返回会话</RouterLink></div>
      <header>
        <div><p class="eyebrow">CONTACT DIRECTORY</p><h1>联系人</h1><p class="subtitle">当前认证账户的可信协作关系。</p></div>
        <span class="mode">只读目录</span>
      </header>
      <p class="boundary" role="note"><strong>关系边界</strong>目录从受认证的服务端响应构建；备注、申请、拉黑与删除操作将在单独的写入切片中接入。</p>

      <section v-if="state === 'loading'" class="state-card" role="status"><i class="spinner" /><h2>正在读取联系人</h2><p>仅加载当前认证账户可见的联系人关系。</p></section>
      <section v-else-if="state === 'unavailable'" class="state-card unavailable" role="alert"><p class="eyebrow">UNAVAILABLE</p><h2>联系人目录暂时不可用</h2><p>已清空旧目录，避免将过期关系当作当前授权结果。</p><button data-contact-retry @click="load">重新确认</button></section>
      <section v-else-if="contacts.length === 0" class="state-card" role="status"><p class="eyebrow">EMPTY</p><h2>还没有可信联系人</h2><p>联系人申请和关系变更将由服务端权威状态决定。</p></section>

      <template v-else>
        <div class="list-heading"><h2>可信联系人 {{ String(contacts.length).padStart(2, '0') }}</h2><span>SERVER OWNED</span></div>
        <div class="contact-list">
          <article v-for="contact in contacts" :key="contact.user.uuid" class="contact-card">
            <div class="avatar" aria-hidden="true">{{ initial(contact) }}</div>
            <div class="identity"><h3>{{ contact.user.nickname }}</h3><p>{{ contact.remark || contact.user.signature || '暂未设置备注' }}</p><small>{{ contact.status === 1 ? '关系已拉黑' : '已验证联系人' }}</small></div>
            <span class="status" :class="{ blocked: contact.status === 1 }">{{ contact.status === 1 ? '已拉黑' : '已验证' }}</span>
          </article>
        </div>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { contactDirectoryClient, type ContactDirectoryClient } from '@/api/contacts'
import type { Contact } from '@/types'
import dipoleLogo from '../../../docs/images/dipole-v3-im-mark-traced.svg'

const props = withDefaults(defineProps<{ client?: ContactDirectoryClient }>(), { client: () => contactDirectoryClient })
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const contacts = ref<Contact[]>([])

onMounted(load)

async function load() {
  state.value = 'loading'
  contacts.value = []
  try {
    contacts.value = await props.client.list()
    state.value = 'ready'
  } catch {
    contacts.value = []
    state.value = 'unavailable'
  }
}

function initial(contact: Contact) { return contact.user.nickname.trim().slice(0, 1).toUpperCase() || '?' }
</script>

<style scoped>
.contact-directory{min-height:100vh;background:var(--dp-canvas);color:var(--dp-ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}.contact-rail{background:var(--dp-rail);color:var(--dp-text-inverse);padding:34px 26px;display:flex;flex-direction:column;gap:12px}.contact-rail strong{font:800 20px var(--dp-font-display);letter-spacing:.11em}.contact-rail>span,.contact-rail p{font:10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-ink-faint)}.contact-rail nav{display:grid;gap:7px;margin-top:24px}.contact-rail nav>*{padding:11px 13px;border-radius:10px;font-size:13px;color:#b7c7c0}.contact-rail nav b{background:var(--dp-rail-soft);color:var(--dp-text-inverse)}.contact-rail p{margin-top:auto;line-height:1.8}main{max-width:1280px;width:100%;margin:auto;padding:42px 54px 68px;box-sizing:border-box}header{display:flex;justify-content:space-between;align-items:center;gap:20px}.eyebrow{font:700 10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-accent-strong);margin:0}.subtitle{color:var(--dp-ink-soft);font-size:14px;margin:6px 0 0}h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:7px 0}.mode{border:1px solid var(--dp-line);background:var(--dp-surface);padding:10px 13px;border-radius:10px;color:var(--dp-accent-strong);font:700 11px var(--dp-font-data)}.boundary{margin:28px 0;padding:14px 17px;border:1px solid var(--dp-accent);background:var(--dp-accent-soft);border-radius:12px;color:var(--dp-ink-soft);font-size:13px}.boundary strong{color:var(--dp-accent-strong);font:700 10px var(--dp-font-data);letter-spacing:.08em;margin-right:14px}.state-card{min-height:360px;background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:16px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:32px}.state-card h2{font:800 22px var(--dp-font-display);margin:12px}.state-card p:not(.eyebrow){color:var(--dp-ink-soft);font-size:13px}.unavailable{border-color:var(--dp-danger)}button{border:0;border-radius:9px;background:var(--dp-rail);color:var(--dp-text-inverse);padding:10px 15px;font:700 12px var(--dp-font-body);cursor:pointer}.spinner{width:23px;height:23px;border:2px solid var(--dp-line);border-top-color:var(--dp-accent);border-radius:50%;animation:spin .8s linear infinite}.list-heading{display:flex;justify-content:space-between;align-items:center;margin:26px 0 13px}.list-heading h2{font:800 12px var(--dp-font-data);letter-spacing:.05em}.list-heading span{font:10px var(--dp-font-data);color:var(--dp-ink-faint)}.contact-list{display:grid;gap:12px}.contact-card{background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:14px;display:flex;gap:15px;align-items:center;padding:20px}.avatar{display:grid;place-items:center;flex:0 0 46px;width:46px;height:46px;border-radius:50%;background:var(--dp-accent-soft);color:var(--dp-accent-strong);font:800 18px var(--dp-font-display)}.identity{min-width:0;flex:1}.identity h3{font:800 18px var(--dp-font-display);margin:0 0 5px}.identity p{color:var(--dp-ink-soft);font-size:13px;margin:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.identity small{display:block;color:var(--dp-ink-faint);font-size:11px;margin-top:6px}.status{border-radius:99px;background:var(--dp-accent-soft);color:var(--dp-accent-strong);padding:7px 10px;font:700 10px var(--dp-font-data)}.status.blocked{background:var(--dp-danger-soft);color:var(--dp-danger)}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:760px){.contact-directory{display:block}.contact-rail{display:none}main{padding:28px 18px 48px}header{align-items:flex-start}h1{font-size:31px}.mode{padding:9px}.boundary{line-height:1.7}.boundary strong{display:block;margin-bottom:5px}.contact-card{padding:16px;align-items:flex-start}.status{white-space:nowrap}.identity h3{font-size:16px}.identity p{white-space:normal;line-height:1.5}}@media(prefers-reduced-motion:reduce){.spinner{animation:none}}
/* V3 brand surface: keep the authenticated directory visually tied to IM. */
.brand-logo{display:block;width:116px;height:86px;object-fit:contain;object-position:center;margin:-8px 0}
.contact-rail>span,.contact-rail p{color:var(--dp-ink-faint)}
.contact-rail nav>*{border-radius:var(--dp-radius-sm);color:var(--dp-ink-faint);text-decoration:none}
.contact-rail nav .active{background:var(--dp-rail-soft);color:var(--dp-text-inverse)}
.mode,.contact-card{border-radius:var(--dp-radius-md)}
.mobile-brandbar{display:none}
@media(max-width:760px){.mobile-brandbar{display:flex;align-items:center;gap:10px;border-bottom:1px solid var(--dp-line);padding:0 0 17px;margin-bottom:28px}.mobile-brandbar img{width:48px;height:36px;object-fit:contain}.mobile-brandbar span{font:800 15px var(--dp-font-display);flex:1}.mobile-brandbar a{color:var(--dp-accent-strong);font-size:12px;font-weight:700;text-decoration:none}}
</style>
