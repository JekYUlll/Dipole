<template>
  <RouterView />
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: var(--dp-font-body); font-size: 14px; background: var(--dp-canvas); color: var(--dp-ink); }
::-webkit-scrollbar { width: 4px; }
::-webkit-scrollbar-thumb { background: var(--dp-ink-faint); border-radius: var(--dp-radius-sm); }
</style>
