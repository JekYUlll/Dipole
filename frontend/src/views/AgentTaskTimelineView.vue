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
.timeline-page { width: min(100% - 2rem, 46rem); margin: 0 auto; padding: 2rem 0; }
</style>
