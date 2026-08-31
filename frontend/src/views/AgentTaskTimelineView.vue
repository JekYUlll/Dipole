<script setup lang="ts">
import { useRoute } from 'vue-router'
import AgentTaskTimeline from '@/components/AgentTaskTimeline.vue'

const route = useRoute()
const agentMark = `${import.meta.env.BASE_URL}dipole-v3-agent.svg`
</script>

<template>
  <main class="timeline-page">
    <div class="timeline-page-orbit" aria-hidden="true"></div>
    <header class="product-bar">
      <RouterLink class="product-lockup" :to="{ name: 'chat' }">
        <img :src="agentMark" alt="Dipole Agent" />
        <span>Dipole Agent</span>
      </RouterLink>
      <span class="product-boundary">DURABLE TASK / READ ONLY</span>
    </header>
    <AgentTaskTimeline :task-id="String(route.params.taskId ?? '')" />
  </main>
</template>

<style scoped>
.timeline-page {
  position: relative;
  box-sizing: border-box;
  min-height: 100vh;
  overflow: hidden;
  padding: clamp(24px, 5vw, 72px) var(--dp-space-md);
  background: var(--dp-canvas);
  color: var(--dp-ink);
  font-family: var(--dp-font-body);
}
.timeline-page > :not(.timeline-page-orbit) { position: relative; z-index: 1; width: min(100%, 52rem); margin-left: auto; margin-right: auto; }
.timeline-page-orbit { position: absolute; top: -210px; right: -100px; width: 560px; height: 280px; border: 1px solid rgba(244, 176, 0, .65); border-radius: 50%; transform: rotate(-28deg); }
.timeline-page-orbit::after { position: absolute; right: 31%; bottom: -5px; width: 14px; height: 14px; border-radius: 50%; background: var(--dp-v3-gold); box-shadow: 0 0 0 6px rgba(244, 176, 0, .15); content: ''; }
.product-bar { display: flex; align-items: center; justify-content: space-between; gap: var(--dp-space-md); margin-bottom: clamp(32px, 6vw, 72px); }
.product-lockup { display: inline-flex; align-items: center; gap: 10px; color: var(--dp-v3-navy); font: 800 1rem/1 var(--dp-font-display); letter-spacing: -.03em; text-decoration: none; }
.product-lockup img { width: 42px; height: 35px; object-fit: contain; }
.product-boundary { color: var(--dp-v3-muted); font: 700 .63rem/1.2 var(--dp-font-data); letter-spacing: .14em; text-align: right; }
@media (max-width: 560px) { .timeline-page { padding-top: 22px; } .product-bar { margin-bottom: 30px; } .product-boundary { max-width: 8rem; } }
</style>
