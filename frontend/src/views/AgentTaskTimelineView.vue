<script setup lang="ts">
import { useRoute } from 'vue-router'
import AgentTaskTimeline from '@/components/AgentTaskTimeline.vue'

const route = useRoute()
</script>

<template>
  <main class="timeline-page">
    <AgentTaskTimeline :task-id="String(route.params.taskId ?? '')" />
  </main>
</template>

<style scoped>
.timeline-page {
  box-sizing: border-box;
  width: min(100%, 46rem);
  min-height: 100vh;
  margin: 0 auto;
  padding: var(--dp-space-lg) var(--dp-space-md);
  background: var(--dp-canvas);
  color: var(--dp-ink);
  font-family: var(--dp-font-body);
}
</style>
