<script setup lang="ts">
export type MessageBubbleVariant = 'self' | 'other' | 'ai' | 'system'

withDefaults(defineProps<{
  variant: MessageBubbleVariant
  senderName?: string
  avatarSrc?: string
  initials?: string
  showSender?: boolean
  media?: boolean
}>(), {
  senderName: '',
  avatarSrc: '',
  initials: '?',
  showSender: false,
  media: false,
})

defineEmits<{
  (e: 'avatar-click'): void
  (e: 'sender-click'): void
}>()
</script>

<template>
  <div class="msg-item" :class="variant" :data-message-variant="variant">
    <div v-if="variant === 'system'" class="msg-system">
      <slot />
    </div>
    <template v-else>
      <button
        v-if="showSender && senderName"
        type="button"
        class="msg-sender-name"
        @click.stop="$emit('sender-click')"
      >{{ senderName }}</button>
      <div class="msg-row">
        <button
          type="button"
          class="msg-avatar"
          :aria-label="senderName || '发送者'"
          @click.stop="$emit('avatar-click')"
        >
          <img v-if="avatarSrc" :src="avatarSrc" :alt="senderName" />
          <span v-else class="msg-avatar-fallback">{{ initials }}</span>
        </button>
        <div class="msg-bubble" :class="{ media }">
          <slot />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.msg-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  width: 100%;
}
.msg-item.self { align-items: flex-end; }
.msg-item.system { align-items: center; }

.msg-sender-name {
  border: 0;
  background: transparent;
  color: var(--dp-ink-soft);
  font: 600 11px var(--dp-font-body);
  padding: 0 44px 0 0;
  cursor: pointer;
}
.msg-item.self .msg-sender-name { padding: 0 0 0 44px; }

.msg-row {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  max-width: 100%;
}
.msg-item.self .msg-row { flex-direction: row-reverse; }

.msg-avatar {
  width: 36px;
  height: 36px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
  flex-shrink: 0;
}
.msg-avatar img,
.msg-avatar-fallback {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  object-fit: cover;
  display: flex;
  align-items: center;
  justify-content: center;
}
.msg-avatar-fallback {
  background: var(--dp-rail-soft);
  color: var(--dp-text-inverse);
  font: 700 13px var(--dp-font-data);
}

.msg-bubble {
  max-width: min(60vw, 480px);
  padding: 10px 14px;
  font: 400 14px/1.5 var(--dp-font-body);
  word-break: break-word;
  white-space: pre-wrap;
}
.msg-item.other .msg-bubble {
  background: var(--dp-surface);
  color: var(--dp-ink);
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-bubble) var(--dp-radius-bubble) var(--dp-radius-bubble) 4px;
}
.msg-item.self .msg-bubble {
  background: var(--dp-rail);
  color: var(--dp-text-inverse);
  border-radius: var(--dp-radius-bubble) var(--dp-radius-bubble) 4px var(--dp-radius-bubble);
}
.msg-item.ai .msg-bubble {
  background: var(--dp-agent-soft);
  color: var(--dp-ink);
  border: 1px solid var(--dp-agent);
  border-left: 3px solid var(--dp-agent);
  border-radius: var(--dp-radius-bubble) var(--dp-radius-bubble) var(--dp-radius-bubble) 4px;
}
.msg-bubble.media {
  padding: 0;
  overflow: hidden;
  max-width: 240px;
  background: transparent;
  border: none;
}

.msg-system {
  max-width: 86%;
  text-align: center;
  color: var(--dp-ink-soft);
  font: 500 12px var(--dp-font-body);
  padding: 6px 12px;
  background: var(--dp-surface-muted);
  border: 1px solid var(--dp-line);
  border-radius: 0;
}
</style>
