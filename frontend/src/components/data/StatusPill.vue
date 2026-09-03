<script setup lang="ts">
// Rectangular BI design rule: **every** container is a rectangle, except
// this pill (999px) and Avatar/Dot (50%). See docs/notes/frontend-bi-redesign.md §4.1.1.
// Renders a dot + label inside a soft-tinted capsule. Purely presentational.

export type StatusTone = 'neutral' | 'agent' | 'accent' | 'success' | 'warning' | 'danger'

const props = withDefaults(defineProps<{
  label: string
  tone?: StatusTone
  /** Show the leading dot. Default true. */
  dot?: boolean
  /** Compact height (18px) vs default (22px). */
  dense?: boolean
}>(), { tone: 'neutral', dot: true, dense: false })
</script>

<template>
  <span
    class="status-pill"
    :class="[`tone-${props.tone}`, { dense: props.dense, 'no-dot': !props.dot }]"
    :data-tone="props.tone"
  >
    <span v-if="props.dot" class="status-pill__dot" aria-hidden="true" />
    <span class="status-pill__label">{{ props.label }}</span>
  </span>
</template>

<style scoped>
/* Only Pill (capsule) and Dot (circle) escape the rectangular rule. */
.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  padding: 0 10px;
  border-radius: var(--dp-radius-pill);
  font: 700 10px var(--dp-font-data);
  letter-spacing: 0.06em;
  white-space: nowrap;
  line-height: 1;
}
.status-pill.dense { height: 18px; padding: 0 8px; font-size: 9px; }
.status-pill.no-dot { padding-left: 10px; }

.status-pill__dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}
.status-pill__label {
  text-transform: uppercase;
}

/* Tones map to Dipole design tokens. */
.status-pill.tone-neutral { background: var(--dp-surface-muted); color: var(--dp-ink-soft); }
.status-pill.tone-agent   { background: var(--dp-agent-soft);   color: var(--dp-agent);   }
.status-pill.tone-accent  { background: var(--dp-accent-soft);  color: var(--dp-accent);  }
.status-pill.tone-success { background: var(--dp-success-soft); color: var(--dp-success); }
.status-pill.tone-warning { background: var(--dp-warning-soft); color: var(--dp-warning); }
.status-pill.tone-danger  { background: var(--dp-danger-soft);  color: var(--dp-danger);  }
</style>
