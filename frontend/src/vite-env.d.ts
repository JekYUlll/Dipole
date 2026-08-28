/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_AGENT_ELICITATION_ENABLED?: string
  readonly VITE_AGENT_SUBSCRIPTIONS_ENABLED?: string
  readonly VITE_AGENT_MEMORIES_ENABLED?: string
  readonly VITE_SEARCH_ENABLED?: string
  readonly VITE_SYNC_ENGINE_MODE?: string
  readonly VITE_SYNC_ENGINE_ENABLED?: string
  readonly VITE_TIMELINE_NOTIFY_MODE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
