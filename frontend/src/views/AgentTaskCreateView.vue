<script setup lang="ts">
import AgentTaskCreate from '@/components/AgentTaskCreate.vue'
</script>

<template>
  <main class="task-create-page"><AgentTaskCreate /></main>
</template>

<style scoped>
.task-create-page { min-height: 100vh; background: var(--dp-canvas); padding: var(--dp-space-md); }
</style>
