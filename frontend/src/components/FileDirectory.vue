<template>
  <section class="file-directory" :data-file-state="state" :aria-busy="state === 'loading'">
    <aside class="file-rail" aria-label="Primary navigation">
      <img class="brand-logo" :src="dipoleLogo" alt="Dipole IM" />
      <span>OWNER WORKSPACE</span>
      <nav><RouterLink :to="{ name: 'chat' }">会话</RouterLink><RouterLink :to="{ name: 'contacts' }">联系人</RouterLink><RouterLink :to="{ name: 'groups' }">群组</RouterLink><RouterLink class="active" :to="{ name: 'files' }">文件</RouterLink><RouterLink :to="{ name: 'devices' }">设备</RouterLink></nav>
      <p>READ ONLY<br />OWNER SCOPED<br />DOWNLOAD ONLY</p>
    </aside>
    <main>
      <div class="mobile-brandbar"><img :src="dipoleLogo" alt="Dipole IM" /><span>文件目录</span><RouterLink :to="{ name: 'chat' }">返回会话</RouterLink></div>
      <header><div><p class="eyebrow">OWNER FILE DIRECTORY</p><h1>文件</h1><p class="subtitle">仅展示当前认证账户上传的文件元数据。</p></div><span class="mode">只读目录</span></header>
      <p class="boundary" role="note"><strong>存储边界</strong>目录不展示对象键、存储 URL、校验值或未完成上传会话；下载仍逐文件重新授权。</p>
      <section v-if="state === 'loading'" class="state-card" role="status"><i class="spinner" /><h2>正在读取文件目录</h2><p>正在确认当前账户的 owner-scoped 文件投影。</p></section>
      <section v-else-if="state === 'unavailable'" class="state-card unavailable" role="alert"><p class="eyebrow">UNAVAILABLE</p><h2>文件目录暂时不可用</h2><p>旧文件已清空，避免将过期目录误认为当前授权结果。</p><button data-file-retry @click="load">重新确认</button></section>
      <section v-else-if="files.length === 0" class="state-card" role="status"><p class="eyebrow">EMPTY</p><h2>还没有可展示的文件</h2><p>文件上传继续在会话编辑器中完成，目录只负责读取和授权下载。</p></section>
      <template v-else>
        <div class="list-heading"><h2>我的文件 {{ String(files.length).padStart(2, '0') }}</h2><span>SERVER OWNED</span></div>
        <div class="file-list"><article v-for="file in files" :key="file.file_id" class="file-card"><div class="file-icon" aria-hidden="true">{{ extension(file) }}</div><div class="identity"><h3>{{ file.file_name }}</h3><p>{{ formatBytes(file.file_size) }} · {{ formatTime(file.created_at) }}</p><small>{{ file.content_type || 'application/octet-stream' }}</small></div><button class="download" :disabled="downloading === file.file_id" @click="download(file)">{{ downloading === file.file_id ? '准备中' : '下载' }}</button></article></div>
        <button v-if="hasMore" class="load-more" :disabled="loadingMore" @click="loadMore">{{ loadingMore ? '读取中...' : '读取更早文件' }}</button>
      </template>
    </main>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ownedFileDirectoryClient, type OwnedFileDirectoryClient, type OwnedFileDirectoryItem } from '@/api/files'
import dipoleLogo from '../../../docs/images/dipole-v3-im-mark-traced.svg'

const props = withDefaults(defineProps<{ client?: OwnedFileDirectoryClient }>(), { client: () => ownedFileDirectoryClient })
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const files = ref<OwnedFileDirectoryItem[]>([])
const nextCursor = ref<string>()
const hasMore = ref(false)
const loadingMore = ref(false)
const downloading = ref<string>()
onMounted(load)

async function load() { state.value = 'loading'; files.value = []; nextCursor.value = undefined; hasMore.value = false; try { const page = await props.client.list(); files.value = page.files; nextCursor.value = page.next_cursor; hasMore.value = page.has_more; state.value = 'ready' } catch { state.value = 'unavailable' } }
async function loadMore() { if (!nextCursor.value || loadingMore.value) return; loadingMore.value = true; try { const page = await props.client.list(nextCursor.value); files.value = [...files.value, ...page.files]; nextCursor.value = page.next_cursor; hasMore.value = page.has_more } catch { files.value = []; nextCursor.value = undefined; hasMore.value = false; state.value = 'unavailable' } finally { loadingMore.value = false } }
async function download(file: OwnedFileDirectoryItem) { downloading.value = file.file_id; try { window.open(await props.client.download(file.file_id), '_blank', 'noopener,noreferrer') } catch { state.value = 'unavailable'; files.value = [] } finally { downloading.value = undefined } }
function extension(file: OwnedFileDirectoryItem) { return file.file_name.split('.').pop()?.slice(0, 4).toUpperCase() || 'FILE' }
function formatBytes(size: number) { return size < 1024 ? `${size} B` : size < 1024 ** 2 ? `${Math.ceil(size / 1024)} KB` : `${(size / 1024 ** 2).toFixed(1)} MB` }
function formatTime(value: string) { return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
</script>

<style scoped>
.file-directory{min-height:100vh;background:var(--dp-canvas);color:var(--dp-ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}.file-rail{background:var(--dp-rail);color:var(--dp-text-inverse);padding:34px 26px;display:flex;flex-direction:column;gap:12px}.brand-logo{display:block;width:116px;height:86px;object-fit:contain;object-position:center;margin:-8px 0}.file-rail>span,.file-rail p{font:10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-ink-faint)}.file-rail nav{display:grid;gap:7px;margin-top:24px}.file-rail nav a{padding:11px 13px;border-radius:var(--dp-radius-sm);font-size:13px;color:var(--dp-ink-faint);text-decoration:none}.file-rail nav .active{background:var(--dp-rail-soft);color:var(--dp-text-inverse)}.file-rail p{margin-top:auto;line-height:1.8}main{max-width:1280px;width:100%;margin:auto;padding:42px 54px 68px;box-sizing:border-box}header{display:flex;justify-content:space-between;align-items:center;gap:20px}.eyebrow{font:700 10px var(--dp-font-data);letter-spacing:.1em;color:var(--dp-accent-strong);margin:0}.subtitle{color:var(--dp-ink-soft);font-size:14px;margin:6px 0 0}h1{font:800 38px var(--dp-font-display);letter-spacing:-.04em;margin:7px 0}.mode{border:1px solid var(--dp-line);background:var(--dp-surface);padding:10px 13px;border-radius:var(--dp-radius-sm);color:var(--dp-accent-strong);font:700 11px var(--dp-font-data)}.boundary{margin:28px 0;padding:14px 17px;border:1px solid var(--dp-accent);background:var(--dp-accent-soft);border-radius:12px;color:var(--dp-ink-soft);font-size:13px}.boundary strong{color:var(--dp-accent-strong);font:700 10px var(--dp-font-data);letter-spacing:.08em;margin-right:14px}.state-card{min-height:360px;background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:16px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:32px}.state-card h2{font:800 22px var(--dp-font-display);margin:12px}.state-card p:not(.eyebrow){color:var(--dp-ink-soft);font-size:13px}.unavailable{border-color:var(--dp-danger)}button{border:0;border-radius:9px;background:var(--dp-rail);color:var(--dp-text-inverse);padding:10px 15px;font:700 12px var(--dp-font-body);cursor:pointer}.spinner{width:23px;height:23px;border:2px solid var(--dp-line);border-top-color:var(--dp-accent);border-radius:50%;animation:spin .8s linear infinite}.list-heading{display:flex;justify-content:space-between;align-items:center;margin:26px 0 13px}.list-heading h2{font:800 12px var(--dp-font-data);letter-spacing:.05em}.list-heading span{color:var(--dp-ink-faint);font:10px var(--dp-font-data)}.file-list{display:grid;gap:12px}.file-card{background:var(--dp-surface);border:1px solid var(--dp-line);border-radius:14px;display:flex;gap:15px;align-items:center;padding:20px}.file-icon{display:grid;place-items:center;flex:0 0 46px;width:46px;height:46px;border-radius:12px;background:var(--dp-accent-soft);color:var(--dp-accent-strong);font:800 10px var(--dp-font-data)}.identity{min-width:0;flex:1}.identity h3{font:800 18px var(--dp-font-display);margin:0 0 5px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.identity p{color:var(--dp-ink-soft);font-size:13px;margin:0}.identity small{display:block;color:var(--dp-ink-faint);font:10px var(--dp-font-data);margin-top:6px}.download{background:var(--dp-accent-strong);white-space:nowrap}.download:disabled,.load-more:disabled{opacity:.6;cursor:wait}.load-more{margin:18px 0 0;background:transparent;color:var(--dp-accent-strong);padding-left:0}@keyframes spin{to{transform:rotate(360deg)}}.mobile-brandbar{display:none}@media(max-width:760px){.file-directory{display:block}.file-rail{display:none}main{padding:28px 18px 48px}header{align-items:flex-start}h1{font-size:31px}.mode{padding:9px}.boundary{line-height:1.7}.boundary strong{display:block;margin-bottom:5px}.file-card{padding:16px;align-items:flex-start}.identity h3{font-size:16px}.identity p{line-height:1.5}.download{padding:9px 10px}.mobile-brandbar{display:flex;align-items:center;gap:10px;border-bottom:1px solid var(--dp-line);padding:0 0 17px;margin-bottom:28px}.mobile-brandbar img{width:48px;height:36px;object-fit:contain}.mobile-brandbar span{font:800 15px var(--dp-font-display);flex:1}.mobile-brandbar a{color:var(--dp-accent-strong);font-size:12px;font-weight:700;text-decoration:none}}@media(prefers-reduced-motion:reduce){.spinner{animation:none}}
</style>
