<script setup lang="ts">
// Inline banner used to replace the `.state-card { margin:72px auto }`
// pattern from the previous IA — see docs/notes/frontend-bi-redesign.md §4.6.
//
// Renders as a full-width 36px horizontal strip with a leading tone icon,
// message text, optional secondary action button, and a close (×) affordance.
// Never occupies the hero position — always sits above a data region so the
// underlying table/skeleton keeps rendering during load/error/stale states.

import { computed } from 'vue'
import {
  IconInfo,
  IconAlertCircle,
  IconCheckCircle,
  IconClose,
} from '@/components/icons'

export type BannerTone = 'info' | 'success' | 'warning' | 'danger'

const props = withDefaults(defineProps<{
  tone?: BannerTone
  message: string
  /** Secondary action label — clicking emits `action`. Omit to hide. */
  actionLabel?: string
  /** Show the close × affordance. Default true. */
  closable?: boolean
}>(), { tone: 'info', closable: true })

const emit = defineEmits<{
  (e: 'action'): void
  (e: 'close'): void
}>()

const iconComponent = computed(() => {
  switch (props.tone) {
    case 'success': return IconCheckCircle
    case 'warning': return IconAlertCircle
    case 'danger':  return IconAlertCircle
    default:        return IconInfo
  }
})
</script>

<template>
  <div class="banner" :class="`tone-${props.tone}`" role="status" :data-tone="props.tone">
    <component :is="iconComponent" :size="14" class="banner__icon" aria-hidden="true" />
    <span class="banner__message">{{ props.message }}</span>
    <span class="banner__spacer" aria-hidden="true" />
    <button
      v-if="props.actionLabel"
      type="button"
      class="banner__action"
      @click="emit('action')"
    >
      {{ props.actionLabel }}
    </button>
    <button
      v-if="props.closable"
      type="button"
      class="banner__close"
      aria-label="关闭"
      @click="emit('close')"
    >
      <IconClose :size="14" />
    </button>
  </div>
</template>

<style scoped>
.banner {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 36px;
  padding: 0 12px;
  border-radius: 0;
  font: 500 12px var(--dp-font-body);
  line-height: 1;
}
.banner__icon { flex-shrink: 0; }
.banner__message { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.banner__spacer { flex: 1; }
.banner__action,
.banner__close {
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: 700 12px var(--dp-font-body);
  padding: 4px 8px;
  border-radius: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.banner__close { padding: 4px; opacity: 0.6; }
.banner__action:hover,
.banner__close:hover { opacity: 1; text-decoration: underline; }
.banner__close:hover { text-decoration: none; }
.banner__close:focus-visible,
.banner__action:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: -2px;
  opacity: 1;
}

.banner.tone-info    { background: var(--dp-surface-muted); color: var(--dp-ink-soft); }
.banner.tone-success { background: var(--dp-success-soft);  color: var(--dp-success);  }
.banner.tone-warning { background: var(--dp-warning-soft);  color: var(--dp-warning);  }
.banner.tone-danger  { background: var(--dp-danger-soft);   color: var(--dp-danger);   }
</style>
