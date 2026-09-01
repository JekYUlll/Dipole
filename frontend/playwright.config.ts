import { defineConfig, devices } from '@playwright/test'

const webkitPreload = process.env.DIPOLE_WEBKIT_LD_PRELOAD
const webkitExecutablePath = process.env.DIPOLE_WEBKIT_EXECUTABLE_PATH

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  reporter: 'line',
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'VITE_AGENT_APPROVAL_ENABLED=true VITE_AGENT_ELICITATION_ENABLED=true VITE_AGENT_SUBSCRIPTIONS_ENABLED=true VITE_AGENT_DEFINITIONS_ENABLED=true VITE_AGENT_MEMORIES_ENABLED=true VITE_AGENT_MEMORY_CORRECTION_ENABLED=true VITE_AGENT_TIMELINE_ENABLED=true VITE_AGENT_ARTIFACTS_ENABLED=true VITE_AGENT_TASK_CREATE_ENABLED=true npm run dev -- --host 127.0.0.1 --port 4173',
    url: 'http://127.0.0.1:4173/app/e2e/indexeddb.html',
    reuseExistingServer: false,
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        launchOptions: webkitPreload || webkitExecutablePath ? {
          ...(webkitExecutablePath ? { executablePath: webkitExecutablePath } : {}),
          ...(webkitPreload ? { env: { LD_PRELOAD: webkitPreload } } : {}),
        } : undefined,
      },
    },
  ],
})
