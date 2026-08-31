<script setup lang="ts">
import AgentTaskCreate from '@/components/AgentTaskCreate.vue'
</script>

<template>
  <main class="task-create-page">
    <div class="task-create-orbit" aria-hidden="true"></div>
    <header class="product-bar">
      <RouterLink class="product-lockup" :to="{ name: 'chat' }">
        <img src="../../../docs/images/dipole-v3-agent.svg" alt="Dipole Agent" />
        <span>Dipole Agent</span>
      </RouterLink>
      <span class="product-boundary">OWNER CONTROL / SHADOW</span>
    </header>
    <AgentTaskCreate />
  </main>
</template>

<style scoped>
.task-create-page { position: relative; min-height: 100vh; overflow: hidden; background: var(--dp-canvas); padding: clamp(24px, 5vw, 72px) var(--dp-space-md); }
.task-create-page > :not(.task-create-orbit) { position: relative; z-index: 1; width: min(100%, 52rem); margin-left: auto; margin-right: auto; }
.task-create-orbit { position: absolute; right: -180px; bottom: -110px; width: 530px; height: 300px; border: 1px solid rgba(244, 176, 0, .56); border-radius: 50%; transform: rotate(21deg); }
.task-create-orbit::after { position: absolute; top: 10%; left: 19%; width: 12px; height: 12px; border-radius: 50%; background: var(--dp-v3-gold); content: ''; }
.product-bar { display: flex; align-items: center; justify-content: space-between; gap: var(--dp-space-md); margin-bottom: clamp(32px, 6vw, 72px); }
.product-lockup { display: inline-flex; align-items: center; gap: 10px; color: var(--dp-v3-navy); font: 800 1rem/1 var(--dp-font-display); letter-spacing: -.03em; text-decoration: none; }
.product-lockup img { width: 35px; height: 35px; }
.product-boundary { color: var(--dp-v3-muted); font: 700 .63rem/1.2 var(--dp-font-data); letter-spacing: .14em; text-align: right; }
@media (max-width: 560px) { .task-create-page { padding-top: 22px; } .product-bar { margin-bottom: 30px; } .product-boundary { max-width: 8rem; } }
</style>
