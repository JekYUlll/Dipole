<script setup lang="ts">
// StatePanel — only used for the *first-paint cold start*, i.e. before we
// know the shape of the data region (no DataTable columns to skeleton).
//
// Once a workspace knows what to render (table + toolbar), it should
// transition to the pattern in docs/notes/frontend-bi-redesign.md §4.6:
// toolbar spinner + skeleton rows + inline Banner. This component is
// deliberately compact so it never becomes the "整屏中央大卡" that we
// just killed.

import ProgressSpinner from 'primevue/progressspinner'
import { IconInfo, IconAlertCircle } from '@/components/icons'

export type StateKind = 'loading' | 'empty' | 'unavailable'

const props = withDefaults(defineProps<{
  state: StateKind
  title?: string
  hint?: string
  /** Optional action label; emits `action` when clicked. */
  actionLabel?: string
}>(), {})

const emit = defineEmits<{ (e: 'action'): void }>()
</script>

<template>
  <div class="state-panel" :class="`state-${props.state}`" role="status">
    <ProgressSpinner
      v-if="props.state === 'loading'"
      class="state-panel__spinner"
      :stroke-width="3"
      aria-label="loading"
    />
    <IconInfo v-else-if="props.state === 'empty'" :size="20" class="state-panel__icon" />
    <IconAlertCircle v-else :size="20" class="state-panel__icon" />

    <div class="state-panel__body">
      <p v-if="props.title" class="state-panel__title">{{ props.title }}</p>
      <p v-if="props.hint" class="state-panel__hint">{{ props.hint }}</p>
    </div>

    <button
      v-if="props.actionLabel"
      type="button"
      class="state-panel__action"
      @click="emit('action')"
    >
      {{ props.actionLabel }}
    </button>
  </div>
</template>

<style scoped>
/* Compact horizontal strip — deliberately NOT the hero of the page.
   BI rule: skeleton/toolbar spinner is preferred; use this only when the
   data region hasn't even mounted yet. */
.state-panel {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--dp-line);
  background: var(--dp-surface);
  border-radius: 0;
  min-height: 56px;
}

.state-panel__spinner { width: 20px; height: 20px; flex-shrink: 0; }
.state-panel__icon    { flex-shrink: 0; }

.state-panel__body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.state-panel__title { font: 600 13px var(--dp-font-body); color: var(--dp-ink); }
.state-panel__hint  { font: 400 12px var(--dp-font-body); color: var(--dp-ink-soft); }

.state-panel__action {
  border: 0;
  background: transparent;
  color: var(--dp-accent);
  font: 700 12px var(--dp-font-body);
  cursor: pointer;
  padding: 4px 8px;
}
.state-panel__action:hover { text-decoration: underline; }

.state-panel.state-loading      { color: var(--dp-ink-soft); }
.state-panel.state-empty        { color: var(--dp-ink-soft); }
.state-panel.state-unavailable  {
  color: var(--dp-danger);
  background: var(--dp-danger-soft);
  border-color: var(--dp-danger);
}
</style>
