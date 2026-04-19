<template>
  <RouterView />
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; font-size: 14px; }
::-webkit-scrollbar { width: 4px; }
::-webkit-scrollbar-thumb { background: #ccc; border-radius: 2px; }
</style>
