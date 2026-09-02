<script setup lang="ts">
import { useRoute } from 'vue-router'
import AgentArtifactMetadata from '@/components/AgentArtifactMetadata.vue'

const route = useRoute()
</script>

<template>
  <AgentArtifactMetadata :artifact-id="String(route.params.artifactId ?? '')" />
</template>
