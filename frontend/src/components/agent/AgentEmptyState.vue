<script setup lang="ts">
// Shared empty-state block for Agent drawer views.
//
// Keeps the `.empty-row` root class so existing test assertions
// (`wrapper.get('.empty-row').text()`) keep working, but replaces the
// bare "还没有 X" one-liner with a compact figure + explanation + CTA
// stack. Rendered inside the drawer body so it lives *inside* the data
// region — never as a hero card that pushes the toolbar down.
//
// The CTA is provided by the caller via the default slot so that
// existing `data-*` selectors (used by tests) can be preserved.

import type { Component } from 'vue'
import { IconInbox } from '@/components/icons'

const props = withDefaults(defineProps<{
  /** Feather-style icon to render inside the figure. */
  icon?: Component
  /** Big first line — should still contain the "还没有 X" phrase for tests. */
  title: string
  /** Longer explanation of what the missing thing is / why to create one. */
  description?: string
}>(), {})
</script>

<template>
  <div class="empty-row" data-agent-empty-state>
    <div class="empty-row__figure" aria-hidden="true">
      <component :is="props.icon ?? IconInbox" :size="22" />
    </div>
    <p class="empty-row__title">{{ props.title }}</p>
    <p v-if="props.description" class="empty-row__description">{{ props.description }}</p>
    <div v-if="$slots.default" class="empty-row__actions"><slot /></div>
  </div>
</template>

<style scoped>
.empty-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 44px 24px;
  text-align: center;
  color: var(--dp-ink-soft);
}
.empty-row__figure {
  width: 44px;
  height: 44px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--dp-surface-muted);
  border: 1px solid var(--dp-line);
  color: var(--dp-ink-faint);
  margin-bottom: 2px;
}
.empty-row__title {
  margin: 0;
  font: 600 13px var(--dp-font-body);
  color: var(--dp-ink);
}
.empty-row__description {
  margin: 0;
  font: 400 12px var(--dp-font-body);
  color: var(--dp-ink-soft);
  line-height: 1.55;
  max-width: 320px;
}
.empty-row__actions {
  margin-top: 6px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
</style>
