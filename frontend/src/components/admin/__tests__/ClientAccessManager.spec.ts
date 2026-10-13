import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ClientAccessManager from '../ClientAccessManager.vue'
import messages from '@/i18n/locales/zh/admin/clientAccess'
import { policyPreview, referencingGroups } from '@/api/admin/clientAccess'
import type { ClientAccessConfig } from '@/api/admin/clientAccess'

const mocks = vi.hoisted(() => ({ get: vi.fn(), save: vi.fn(), groups: vi.fn(), success: vi.fn() }))
vi.mock('@/api/admin/clientAccess', async importOriginal => ({ ...(await importOriginal<object>()), getClientAccess: mocks.get, saveClientAccess: mocks.save }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: mocks.groups }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.success }) }))
vi.mock('vue-i18n', async importOriginal => ({ ...(await importOriginal<object>()), useI18n: () => ({ t: (key: string, params: Record<string, string | number> = {}) => {
  const field = key.replace('admin.clientAccess.', '') as keyof typeof messages.clientAccess
  let text = messages.clientAccess[field] ?? key
  for (const [name, value] of Object.entries(params)) text = text.replace(`{${name}}`, String(value))
  return text
} }) }))
enableAutoUnmount(afterEach)

function config(): ClientAccessConfig {
  return { revision: 'r1', clients: [
    { id: 'codex', name: 'Codex 官方客户端', enabled: true, detector: 'codex', match: 'all', rules: [] },
    { id: 'deepseek_harness', name: 'DeepSeek Harness', enabled: true, detector: 'headers', match: 'all', rules: [{ header: 'User-Agent', operator: 'prefix', value: 'deepseek-harness/' }] }
  ], groups: { '38': { mode: 'allow', client_ids: ['codex'] } } }
}
beforeEach(() => {
  vi.resetAllMocks()
  mocks.get.mockImplementation(async () => config())
  mocks.groups.mockResolvedValue([{ id: 38, name: '开发组' }, { id: 99, name: '另一组' }])
  mocks.save.mockResolvedValue(undefined)
})
async function open() {
  const wrapper = mount(ClientAccessManager, { props: { groupId: '38' } })
  await flushPromises()
  return wrapper
}

describe('ClientAccessManager', () => {
  it('loads the selected group and renders dynamic names', async () => {
    const wrapper = await open()
    expect(wrapper.text()).toContain('当前 API Key 所属分组仅允许使用：Codex 官方客户端')
    expect(wrapper.get('[data-testid="save-client-access"]').attributes('disabled')).toBeDefined()
    const codex = wrapper.findAll('details')[0]
    expect(codex.find('input[type="checkbox"]').attributes('disabled')).toBeDefined()
    expect(codex.find('button').attributes('disabled')).toBeDefined()
  })
  it('supports multi-select deny mode and preserves other group policies', async () => {
    const wrapper = await open()
    await wrapper.get('select[aria-label]').setValue('deny')
    const selection = wrapper.findAll('section.card')[1].findAll('input[type="checkbox"]')
    await selection[1].setValue(true)
    expect(wrapper.text()).toContain('禁止使用：Codex 官方客户端、DeepSeek Harness')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledTimes(1)
    expect(mocks.save.mock.calls[0][0].groups['38']).toEqual({ mode: 'deny', client_ids: ['codex', 'deepseek_harness'] })
    expect(mocks.get).toHaveBeenCalledTimes(2)
  })
  it('rejects an empty selection without sending a write', async () => {
    const wrapper = await open()
    const selection = wrapper.findAll('section.card')[1].findAll('input[type="checkbox"]')
    await selection[0].setValue(false)
    await wrapper.get('form').trigger('submit')
    expect(mocks.save).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('至少选择一个')
  })
  it('keeps edits on a concurrency conflict', async () => {
    mocks.save.mockRejectedValue({ status: 409 })
    const wrapper = await open()
    await wrapper.get('select[aria-label]').setValue('deny')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('其他管理员')
    expect(wrapper.get('select[aria-label]').element).toHaveProperty('value', 'deny')
    expect(mocks.get).toHaveBeenCalledTimes(1)
  })
  it('shows load failures without an editable default configuration', async () => {
    mocks.get.mockRejectedValue(new Error('offline'))
    const wrapper = await open()
    expect(wrapper.get('[role="alert"]').text()).toContain('加载失败')
    expect(wrapper.find('form').exists()).toBe(false)
  })
  it('supports adding a custom client with header conditions', async () => {
    const wrapper = await open()
    await wrapper.findAll('section.card')[0].get('button').trigger('click')
    const custom = wrapper.findAll('details')[2]
    const inputs = custom.findAll('input:not([type="checkbox"])')
    await inputs[0].setValue('company_tool')
    await inputs[1].setValue('公司工具')
    await inputs[2].setValue('X-Company-Client')
    await inputs[3].setValue('company/')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save.mock.calls[0][0].clients[2]).toMatchObject({ id: 'company_tool', name: '公司工具', rules: [{ header: 'X-Company-Client', operator: 'prefix', value: 'company/' }] })
  })
  it('uses configuration names and all references, not hardcoded group text', () => {
    const cfg = config()
    cfg.clients[0].name = '新名称'
    expect(policyPreview(cfg.groups['38'], cfg.clients)).toBe('新名称')
    expect(referencingGroups(cfg, 'codex')).toEqual(['38'])
    expect(referencingGroups(cfg, 'deepseek_harness')).toEqual([])
  })
})
