<template>
  <section class="group-directory" :data-group-state="state" :aria-busy="state === 'loading'">
    <aside class="group-rail" aria-label="Primary navigation">
      <img class="brand-logo" :src="dipoleLogo" alt="Dipole IM" />
      <span>GROUPS / SIGNAL</span>
      <nav>
        <RouterLink :to="{ name: 'chat' }">会话</RouterLink>
        <RouterLink class="active" :to="{ name: 'groups' }">协作群组</RouterLink>
        <RouterLink :to="{ name: 'contacts' }">联系人</RouterLink>
      </nav>
      <p>READ ONLY<br />AUTHENTICATED SCOPE</p>
    </aside>

    <main>
      <div class="mobile-brandbar"><img :src="dipoleLogo" alt="Dipole IM" /><span>协作群组</span><RouterLink :to="{ name: 'chat' }">返回会话</RouterLink></div>
      <header>
        <div><p class="eyebrow">GROUP DIRECTORY</p><h1>协作群组</h1><p class="subtitle">当前认证账户可见的群组与实时同步边界。</p></div>
        <span class="mode">只读目录</span>
      </header>
      <p class="boundary" role="note"><strong>成员边界</strong>目录只读取服务端权威投影；成员邀请、群资料修改与解散操作保持关闭。</p>

      <section v-if="state === 'loading'" class="state-card" role="status"><i class="spinner" /><h2>正在确认群组</h2><p>仅加载当前认证账户可见的群投影。</p></section>
      <section v-else-if="state === 'unavailable'" class="state-card unavailable" role="alert"><p class="eyebrow">UNAVAILABLE</p><h2>群目录暂时不可用</h2><p>已清空旧目录，避免将过期群状态当作当前授权结果。</p><button data-group-retry type="button" @click="load">重新确认</button></section>
      <section v-else-if="groups.length === 0" class="state-card" role="status"><p class="eyebrow">EMPTY</p><h2>暂无可见群组</h2><p>创建和邀请操作继续在经过授权的写入切片中处理。</p></section>

      <template v-else>
        <div class="list-heading"><h2>可见群组 {{ String(groups.length).padStart(2, '0') }}</h2><span>HOT GROUPS USE NOTIFY + PULL</span></div>
        <div class="group-list" aria-label="可见群组">
          <article v-for="group in groups" :key="group.uuid" class="group-card">
            <div class="avatar" aria-hidden="true">{{ initial(group) }}</div>
            <div class="identity"><h3>{{ group.name }}</h3><p>{{ group.member_count }} 位成员 <template v-if="group.owner">/ 群主：{{ group.owner.nickname }}</template></p><small v-if="group.status === 1">群聊已解散，保留只读历史边界</small></div>
            <span class="status" :class="{ hot: group.is_hot, dismissed: group.status === 1 }">{{ label(group) }}</span>
          </article>
        </div>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { groupDirectoryClient, type GroupDirectoryClient } from '@/api/groups'
import type { Group } from '@/types'
import dipoleLogo from '../../../docs/images/dipole-v3-im-mark-traced.svg'

const props = withDefaults(defineProps<{ client?: GroupDirectoryClient }>(), { client: () => groupDirectoryClient })
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const groups = ref<Group[]>([])

onMounted(load)

async function load() {
  state.value = 'loading'
  groups.value = []
  try {
    groups.value = await props.client.list()
    state.value = 'ready'
  } catch {
    groups.value = []
    state.value = 'unavailable'
  }
}

function initial(group: Group) { return group.name.trim().slice(0, 1).toUpperCase() || '?' }
function label(group: Group) { return group.status === 1 ? '已解散' : group.is_hot ? 'Hot / Pull' : '已同步' }
</script>

<style scoped>
.group-directory{min-height:100vh;background:var(--dp-canvas);color:var(--dp-ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}
.group-rail{background:var(--dp-rail);color:var(--dp-text-inverse);padding:34px 26px;display:flex;flex-direction:column;gap:12px}.brand-logo{display:block;width:116px;height:86px;object-fit:contain;object-position:center;margin:-8px 0}.group-rail>span,.group-rail p{font:10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-ink-faint)}.group-rail nav{display:grid;gap:7px;margin-top:24px}.group-rail nav>*{padding:11px 13px;border-radius:var(--dp-radius-sm);font-size:13px;color:var(--dp-ink-faint);text-decoration:none}.group-rail nav .active{background:var(--dp-rail-soft);color:var(--dp-text-inverse)}.group-rail p{margin-top:auto;line-height:1.8}
main{max-width:1280px;width:100%;margin:auto;padding:42px 54px 68px;box-sizing:border-box}header{display:flex;justify-content:space-between;align-items:center;gap:20px}.eyebrow{font:700 10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-accent-strong);margin:0}.subtitle{color:var(--dp-ink-soft);font-size:14px;margin:6px 0 0}h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:7px 0}.mode{border:1px solid var(--dp-line);background:var(--dp-surface);padding:10px 13px;border-radius:var(--dp-radius-md);color:var(--dp-accent-strong);font:700 11px var(--dp-font-data)}.boundary{margin:28px 0;padding:14px 17px;border:1px solid var(--dp-accent);background:var(--dp-accent-soft);border-radius:12px;color:var(--dp-ink-soft);font-size:13px}.boundary strong{color:var(--dp-accent-strong);font:700 10px var(--dp-font-data);letter-spacing:.08em;margin-right:14px}.state-card{min-height:360px;background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:16px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:32px}.state-card h2{font:800 22px var(--dp-font-display);margin:12px}.state-card p:not(.eyebrow){color:var(--dp-ink-soft);font-size:13px}.unavailable{border-color:var(--dp-danger)}button{border:0;border-radius:9px;background:var(--dp-rail);color:var(--dp-text-inverse);padding:10px 15px;font:700 12px var(--dp-font-body);cursor:pointer}.spinner{width:23px;height:23px;border:2px solid var(--dp-line);border-top-color:var(--dp-accent);border-radius:50%;animation:spin .8s linear infinite}.list-heading{display:flex;justify-content:space-between;align-items:center;margin:26px 0 13px}.list-heading h2{font:800 12px var(--dp-font-data);letter-spacing:.05em}.list-heading span{font:10px var(--dp-font-data);color:var(--dp-ink-faint)}.group-list{display:grid;gap:12px}.group-card{background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:var(--dp-radius-md);display:flex;gap:15px;align-items:center;padding:20px}.avatar{display:grid;place-items:center;flex:0 0 46px;width:46px;height:46px;border-radius:50%;background:var(--dp-accent-soft);color:var(--dp-accent-strong);font:800 18px var(--dp-font-display)}.identity{min-width:0;flex:1}.identity h3{font:800 18px var(--dp-font-display);margin:0 0 5px}.identity p{color:var(--dp-ink-soft);font-size:13px;margin:0}.identity small{display:block;color:var(--dp-danger);font-size:11px;margin-top:6px}.status{border-radius:99px;background:var(--dp-accent-soft);color:var(--dp-accent-strong);padding:7px 10px;font:700 10px var(--dp-font-data);white-space:nowrap}.status.hot{background:var(--dp-warning-soft);color:var(--dp-warning)}.status.dismissed{background:var(--dp-danger-soft);color:var(--dp-danger)}@keyframes spin{to{transform:rotate(360deg)}}
.mobile-brandbar{display:none}@media(max-width:760px){.group-directory{display:block}.group-rail{display:none}main{padding:28px 18px 48px}header{align-items:flex-start}h1{font-size:31px}.mode{padding:9px}.boundary{line-height:1.7}.boundary strong{display:block;margin-bottom:5px}.group-card{padding:16px;align-items:flex-start}.identity h3{font-size:16px}.list-heading{align-items:flex-start;gap:8px;flex-direction:column}.mobile-brandbar{display:flex;align-items:center;gap:10px;border-bottom:1px solid var(--dp-line);padding:0 0 17px;margin-bottom:28px}.mobile-brandbar img{width:48px;height:36px;object-fit:contain}.mobile-brandbar span{font:800 15px var(--dp-font-display);flex:1}.mobile-brandbar a{color:var(--dp-accent-strong);font-size:12px;font-weight:700;text-decoration:none}}@media(prefers-reduced-motion:reduce){.spinner{animation:none}}
</style>
