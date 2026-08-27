<template>
  <main class="search-workspace" aria-labelledby="message-search-title">
    <header class="search-header">
      <button class="search-back" type="button" aria-label="关闭消息搜索" @click="emit('close')">
        <IconBack :size="20" />
      </button>
      <div>
        <h1 id="message-search-title">搜索消息</h1>
        <p>仅搜索你有权访问的会话</p>
      </div>
    </header>

    <section class="search-content">
      <label class="search-field">
        <IconSearch :size="20" />
        <input
          ref="inputRef"
          v-model="searchQuery"
          type="search"
          autocomplete="off"
          placeholder="搜索消息内容"
          aria-label="搜索消息内容"
        />
        <kbd>⌘ K</kbd>
      </label>

      <div class="search-toolbar">
        <strong>{{ resultSummary }}</strong>
        <span>范围随会话权限实时更新</span>
      </div>

      <div class="search-body" aria-live="polite" :aria-busy="status === 'loading'">
        <div v-if="status === 'idle'" class="search-state">
          <div class="state-icon"><IconSearch :size="24" /></div>
          <h2>开始搜索</h2>
          <p>查找你有权限访问的消息，结果不会扩大现有会话范围。</p>
        </div>

        <div v-else-if="status === 'loading'" class="search-list" aria-label="正在搜索">
          <div v-for="index in 4" :key="index" class="search-skeleton">
            <span class="skeleton-avatar" />
            <span class="skeleton-lines"><i /><i /><i /></span>
          </div>
        </div>

        <div v-else-if="status === 'empty'" class="search-state">
          <div class="state-icon"><IconXCircle :size="24" /></div>
          <h2>没有找到相关消息</h2>
          <p>试试更短的关键词，或换一个更明确的表达。</p>
        </div>

        <div v-else-if="status === 'error'" class="search-state error-state">
          <div class="state-icon"><IconAlertCircle :size="24" /></div>
          <h2>搜索服务未响应</h2>
          <p>聊天仍可正常使用。请稍后重试本次搜索。</p>
          <button type="button" data-search-retry @click="retry">重新尝试</button>
        </div>

        <div v-else class="search-list">
          <button
            v-for="message in results"
            :key="message.message_id"
            type="button"
            class="search-result"
            :data-search-result="message.message_id"
            @click="emit('select', message)"
          >
            <span class="result-avatar">{{ conversationInitial(message.conversation_key) }}</span>
            <span class="result-main">
              <span class="result-meta">
                <strong>{{ conversationName(message.conversation_key) }}</strong>
                <span>#{{ message.message_seq }}</span>
                <time :datetime="message.sent_at">{{ formatTime(message.sent_at) }}</time>
              </span>
              <span class="result-content">{{ message.content }}</span>
              <span class="result-sender">来自 {{ senderName(message) }}</span>
            </span>
            <span class="result-arrow" aria-hidden="true">→</span>
          </button>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { IconAlertCircle, IconBack, IconSearch, IconXCircle } from '@/components/icons'
import { searchMessages } from '@/api/search'
import { useMessageSearch, type MessageSearch } from '@/composables/useMessageSearch'
import type { Conversation, PublicUser, SearchMessageResult } from '@/types'

const props = withDefaults(defineProps<{
  conversations: Conversation[]
  currentUser?: PublicUser
  initialQuery?: string
  searcher?: MessageSearch
}>(), {
  initialQuery: '',
  searcher: undefined,
})

const emit = defineEmits<{
  close: []
  select: [message: SearchMessageResult]
}>()

const inputRef = ref<HTMLInputElement | null>(null)
const executeSearch: MessageSearch = (text, limit) => (props.searcher ?? searchMessages)(text, limit)
const { query, results, status, retry } = useMessageSearch(executeSearch)
const searchQuery = computed({
  get: () => query.value,
  set: (value: string) => { query.value = [...value].slice(0, 256).join('') },
})

const conversationByKey = computed(() => new Map(
  props.conversations.map(conversation => [conversation.conversation_key, conversation])
))

const conversationName = (key: string) => {
  const conversation = conversationByKey.value.get(key)
  if (conversation?.remark) return conversation.remark
  if (conversation?.target_type === 1) return conversation.target_group?.name || key.replace('group:', '')
  return conversation?.target_user?.nickname || key
}

const conversationInitial = (key: string) => conversationName(key).trim().slice(0, 1).toUpperCase() || '?'

const senderName = (message: SearchMessageResult) => {
  if (props.currentUser?.uuid === message.from_uuid) return '我'
  const conversation = conversationByKey.value.get(message.conversation_key)
  if (conversation?.target_user?.uuid === message.from_uuid) return conversation.target_user.nickname
  const member = conversation?.target_group?.members?.find(item => item.user.uuid === message.from_uuid)
  return member?.user.nickname || message.from_uuid
}

const formatTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false,
  }).format(date)
}

const resultSummary = computed(() => {
  if (status.value === 'loading') return '正在搜索…'
  if (status.value === 'results') return `${results.value.length} 条结果`
  if (status.value === 'empty') return '0 条结果'
  if (status.value === 'error') return '搜索暂时不可用'
  return '输入关键词开始搜索'
})

onMounted(() => {
  query.value = props.initialQuery.trim()
  nextTick(() => inputRef.value?.focus())
})
</script>

<style scoped>
.search-workspace {
  --search-canvas: #f3f1ea;
  --search-surface: #fffefb;
  --search-ink: #17211d;
  --search-soft: #66716c;
  --search-line: #dde2dc;
  --search-accent: #00a86b;
  flex: 1;
  min-width: 0;
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  color: var(--search-ink);
  background:
    radial-gradient(circle at 82% 10%, rgba(0, 168, 107, 0.08), transparent 26rem),
    var(--search-canvas);
  font-family: Manrope, "Noto Sans SC", sans-serif;
}

.search-header {
  min-height: 76px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 40px;
  border-bottom: 1px solid rgba(23, 33, 29, 0.08);
  background: rgba(255, 254, 251, 0.78);
  backdrop-filter: blur(14px);
}

.search-header h1 { margin: 0; font-size: 20px; letter-spacing: -0.02em; }
.search-header p { margin: 3px 0 0; color: var(--search-soft); font-size: 12px; }

.search-back {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  border: 1px solid var(--search-line);
  border-radius: 12px;
  color: var(--search-ink);
  background: var(--search-surface);
  cursor: pointer;
}

.search-content {
  width: min(920px, calc(100% - 80px));
  margin: 0 auto;
  padding: 44px 0 32px;
  overflow: auto;
}

.search-field {
  height: 58px;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 18px;
  border: 1px solid var(--search-line);
  border-radius: 16px;
  background: var(--search-surface);
  box-shadow: 0 18px 50px rgba(23, 33, 29, 0.08);
  color: var(--search-soft);
}

.search-field:focus-within { border-color: var(--search-accent); box-shadow: 0 0 0 3px rgba(0, 168, 107, 0.13); }
.search-field input { flex: 1; min-width: 0; border: 0; outline: 0; background: transparent; color: var(--search-ink); font: inherit; font-size: 15px; }
.search-field kbd { padding: 4px 8px; border: 1px solid var(--search-line); border-radius: 7px; color: var(--search-soft); background: #f4f5f2; font: 600 11px/1.2 Manrope, sans-serif; }

.search-toolbar { display: flex; justify-content: space-between; align-items: center; padding: 20px 2px 12px; }
.search-toolbar strong { font-size: 13px; }
.search-toolbar span { color: #98a19d; font-size: 11px; }

.search-body { min-height: 440px; }
.search-list { display: flex; flex-direction: column; gap: 10px; }

.search-result {
  width: 100%;
  display: flex;
  align-items: flex-start;
  gap: 15px;
  padding: 18px;
  border: 1px solid var(--search-line);
  border-radius: 15px;
  text-align: left;
  color: var(--search-ink);
  background: var(--search-surface);
  cursor: pointer;
  transition: transform 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.search-result:hover, .search-result:focus-visible { transform: translateY(-2px); border-color: rgba(0, 168, 107, 0.48); box-shadow: 0 14px 36px rgba(23, 33, 29, 0.08); outline: none; }
.result-avatar { width: 42px; height: 42px; flex: 0 0 42px; display: grid; place-items: center; border-radius: 13px; color: #007a4e; background: #ddf5e9; font-weight: 800; }
.result-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 7px; }
.result-meta { display: flex; align-items: center; gap: 10px; font-size: 12px; color: var(--search-soft); }
.result-meta strong { color: var(--search-ink); font-size: 13px; }
.result-meta time { margin-left: auto; }
.result-content { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
.result-sender { color: var(--search-soft); font-size: 11px; }
.result-arrow { align-self: center; color: var(--search-accent); font-size: 19px; }

.search-state { min-height: 420px; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 36px; border: 1px solid var(--search-line); border-radius: 18px; text-align: center; background: var(--search-surface); }
.state-icon { width: 54px; height: 54px; display: grid; place-items: center; border-radius: 17px; color: #007a4e; background: #ddf5e9; }
.search-state h2 { margin: 18px 0 8px; font-size: 21px; }
.search-state p { max-width: 360px; margin: 0; color: var(--search-soft); font-size: 13px; line-height: 1.7; }
.error-state .state-icon { color: #d94c4c; background: #fde5e2; }
.error-state button { margin-top: 20px; padding: 10px 16px; border: 0; border-radius: 10px; color: white; background: #172126; cursor: pointer; font-weight: 700; }

.search-skeleton { min-height: 112px; display: flex; gap: 15px; padding: 18px; box-sizing: border-box; border: 1px solid var(--search-line); border-radius: 15px; background: var(--search-surface); }
.skeleton-avatar { width: 42px; height: 42px; flex: 0 0 42px; border-radius: 13px; background: #e8ece8; }
.skeleton-lines { flex: 1; display: flex; flex-direction: column; gap: 11px; padding-top: 2px; }
.skeleton-lines i { height: 10px; border-radius: 999px; background: linear-gradient(90deg, #e8ece8, #f4f5f2, #e8ece8); background-size: 200% 100%; animation: search-shimmer 1.4s linear infinite; }
.skeleton-lines i:nth-child(1) { width: 38%; }
.skeleton-lines i:nth-child(3) { width: 54%; }

@keyframes search-shimmer { to { background-position: -200% 0; } }

@media (max-width: 720px) {
  .search-workspace { position: absolute; inset: 0; z-index: 20; }
  .search-header { min-height: 64px; padding: 0 16px; }
  .search-header h1 { font-size: 18px; }
  .search-content { width: auto; margin: 0; padding: 22px 16px 28px; }
  .search-field { height: 54px; border-radius: 14px; }
  .search-field kbd, .search-toolbar span { display: none; }
  .search-toolbar { padding-top: 18px; }
  .search-state { min-height: 430px; padding: 24px; }
  .search-result { padding: 16px; gap: 12px; }
  .result-meta { flex-wrap: wrap; gap: 7px; }
  .result-meta time { margin-left: auto; }
  .result-content { white-space: normal; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
}

@media (prefers-reduced-motion: reduce) {
  .search-result { transition: none; }
  .skeleton-lines i { animation: none; }
}
</style>
