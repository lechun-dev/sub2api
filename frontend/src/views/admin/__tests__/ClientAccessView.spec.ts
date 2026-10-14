import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ClientAccessView from '../ClientAccessView.vue'
import messages from '@/i18n/locales/zh/admin/clientAccess'
import type { ClientAccessConfig } from '@/api/admin/clientAccess'

const mocks = vi.hoisted(() => ({ get: vi.fn(), groups: vi.fn() }))
vi.mock('@/api/admin/clientAccess', async importOriginal => ({ ...(await importOriginal<object>()), getClientAccess: mocks.get }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: mocks.groups }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', async importOriginal => ({ ...(await importOriginal<object>()), useI18n: () => ({ t: (key: string, params: Record<string, string | number> = {}) => {
  const field = key.replace('admin.clientAccess.', '') as keyof typeof messages.clientAccess
  let text = messages.clientAccess[field] ?? key
  for (const [name, value] of Object.entries(params)) text = text.replace(`{${name}}`, String(value))
  return text
} }) }))
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  mocks.get.mockImplementation(async (): Promise<ClientAccessConfig> => ({ revision: 'r1', clients: [
    { id: 'codex', name: 'Codex', enabled: true, detector: 'codex', match: 'all', rules: [] }
  ], groups: { '38': { mode: 'allow', client_ids: ['codex'] } } }))
  mocks.groups.mockResolvedValue([{ id: 38, name: '开发组' }, { id: 99, name: '另一组' }])
  vi.spyOn(window, 'confirm').mockReturnValue(false)
})
afterEach(() => vi.restoreAllMocks())
async function open(url = '/admin/client-access?group=38') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/admin/client-access', component: ClientAccessView },
    { path: '/admin/groups', component: { template: '<p>分组列表</p>' } }
  ] })
  await router.push(url)
  await router.isReady()
  const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [router] } })
  await flushPromises()
  return { router, wrapper }
}
describe('ClientAccessView route context', () => {
  it('reloads the correct group when navigating to a different group on the same route', async () => {
    const { router, wrapper } = await open()
    await router.push('/admin/client-access?group=99')
    await flushPromises()
    expect(wrapper.get('h1').text()).toContain('另一组')
    expect(wrapper.get('[data-testid="policy-mode"]').element).toHaveProperty('value', 'off')
    expect(mocks.get).toHaveBeenCalledTimes(2)
  })
  it('blocks query changes when the current group draft is not discarded', async () => {
    const { router, wrapper } = await open()
    await wrapper.get('[data-testid="policy-mode"]').setValue('deny')
    await router.push('/admin/client-access?group=99')
    await flushPromises()
    expect(window.confirm).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.query.group).toBe('38')
    expect(wrapper.get('h1').text()).toContain('开发组')
    expect(wrapper.get('[data-testid="policy-mode"]').element).toHaveProperty('value', 'deny')
  })
  it('allows a confirmed discard and resets the editor for the new group', async () => {
    const { router, wrapper } = await open()
    await wrapper.get('[data-testid="policy-mode"]').setValue('deny')
    vi.mocked(window.confirm).mockReturnValue(true)
    await router.push('/admin/client-access?group=99')
    await flushPromises()
    expect(wrapper.get('h1').text()).toContain('另一组')
    expect(wrapper.get('[data-testid="policy-mode"]').element).toHaveProperty('value', 'off')
  })
  it('opens the separate client registry from a group without a group picker', async () => {
    const { wrapper } = await open()
    await wrapper.get('a[href="/admin/client-access?tab=clients"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('details').exists()).toBe(true)
    expect(wrapper.find('[data-testid="policy-mode"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('保存客户端规则')
  })
  it('guards leaving for the group list with unsaved edits', async () => {
    const { router, wrapper } = await open()
    await wrapper.get('[data-testid="policy-mode"]').setValue('deny')
    await wrapper.get('a[href="/admin/groups"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/client-access')
    expect(window.confirm).toHaveBeenCalledTimes(1)
  })
  it.each(['/admin/client-access?group', '/admin/client-access?group=38&group=99'])('does not turn a malformed group link into global mode: %s', async url => {
    const { wrapper } = await open(url)
    expect(wrapper.get('[role="alert"]').text()).toContain('不存在或不可访问')
    expect(wrapper.find('[data-testid="add-policy"]').exists()).toBe(false)
  })
})
