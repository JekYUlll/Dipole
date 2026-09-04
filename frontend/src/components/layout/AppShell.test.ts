import { describe, expect, it, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import AppShell from './AppShell.vue'
import { useAuthStore } from '@/stores/auth'

const makeRouter = () => createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', component: { template: '<div/>' } },
  ],
})

describe('AppShell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.currentUser = { uuid: 'U-evan', nickname: 'Evan', avatar: '' } as any
  })

  it('exposes only one Chat workspace tab; Settings and Directory are not duplicated in the topbar', () => {
    const router = makeRouter()
    const w = mount(AppShell, { global: { plugins: [router] } })
    const tabs = w.findAll('.app-shell__tab').map(t => t.text())
    expect(tabs).toEqual(['Chat'])
    // 设置入口只有一个,并且不是 tab 或 RouterLink,而是发 event 的按钮。
    const settingsEntries = w.findAll('[data-open-settings]')
    expect(settingsEntries).toHaveLength(1)
    expect(w.findAll('a').every(a => a.attributes('href') !== '/settings')).toBe(true)
  })

  it('marks the active workspace tab', () => {
    const router = makeRouter()
    const w = mount(AppShell, {
      global: { plugins: [router] },
      props: { activeWorkspace: 'chat' },
    })
    const active = w.findAll('.app-shell__tab--active')
    expect(active).toHaveLength(1)
    expect(active[0].text()).toBe('Chat')
  })

  it('renders agent pending badge only when count > 0', async () => {
    const router = makeRouter()
    const w1 = mount(AppShell, { global: { plugins: [router] }, props: { agentPending: 0 } })
    expect(w1.find('.app-shell__agent-badge').exists()).toBe(false)
    const w2 = mount(AppShell, { global: { plugins: [router] }, props: { agentPending: 3 } })
    expect(w2.find('.app-shell__agent-badge').text()).toBe('3')
    const w3 = mount(AppShell, { global: { plugins: [router] }, props: { agentPending: 137 } })
    expect(w3.find('.app-shell__agent-badge').text()).toBe('99+')
  })

  it('toggles agent drawer by mutating URL query and emits event', async () => {
    const router = makeRouter()
    await router.push('/')
    await router.isReady()
    const w = mount(AppShell, { global: { plugins: [router] } })
    await w.find('[data-agent-toggle]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.agent).toBe('1')
    expect(router.currentRoute.value.query.view).toBe('live')
    expect(w.emitted('toggle-agent')).toHaveLength(1)
    await w.find('[data-agent-toggle]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.agent).toBeUndefined()
    expect(router.currentRoute.value.query.view).toBeUndefined()
  })

  it('marks agent toggle active when prop set', () => {
    const router = makeRouter()
    const w = mount(AppShell, { global: { plugins: [router] }, props: { agentActive: true } })
    expect(w.find('.app-shell__agent-toggle--active').exists()).toBe(true)
    expect(w.find('[data-agent-toggle]').attributes('aria-pressed')).toBe('true')
  })

  it('emits open-settings from the gear icon and avatar', async () => {
    const router = makeRouter()
    const w = mount(AppShell, { global: { plugins: [router] } })
    await w.find('[data-open-settings]').trigger('click')
    await w.find('.app-shell__avatar').trigger('click')
    expect(w.emitted('open-settings')).toHaveLength(2)
  })

  it('renders status bar with default caption', () => {
    const router = makeRouter()
    const w = mount(AppShell, { global: { plugins: [router] }, props: { statusText: 'SHADOW · 12 subs' } })
    expect(w.find('.app-shell__statusbar').text()).toContain('SHADOW · 12 subs')
  })

  it('hides status bar when showStatusBar=false', () => {
    const router = makeRouter()
    const w = mount(AppShell, { global: { plugins: [router] }, props: { showStatusBar: false } })
    expect(w.find('.app-shell__statusbar').exists()).toBe(false)
  })

  it('exposes default and agent-drawer slots', () => {
    const router = makeRouter()
    const w = mount(AppShell, {
      global: { plugins: [router] },
      slots: {
        default: '<div class="test-main">main</div>',
        'agent-drawer': '<div class="test-drawer">drawer</div>',
      },
    })
    expect(w.find('.test-main').exists()).toBe(true)
    expect(w.find('.test-drawer').exists()).toBe(true)
  })
})
