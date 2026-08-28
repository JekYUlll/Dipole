<template>
  <AgentElicitationForm :task-id="taskId" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AgentElicitationForm from '@/components/AgentElicitationForm.vue'

const route = useRoute()
const taskId = computed(() => String(route.params.taskId ?? ''))
</script>
