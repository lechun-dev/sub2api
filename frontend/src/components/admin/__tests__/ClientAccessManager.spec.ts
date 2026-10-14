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
  let stored = config()
  mocks.get.mockImplementation(async () => structuredClone(stored))
  mocks.groups.mockResolvedValue([{ id: 38, name: '开发组' }, { id: 99, name: '另一组' }])
  mocks.save.mockImplementation(async (next: ClientAccessConfig) => { stored = structuredClone({ ...next, revision: 'r2' }) })
  vi.spyOn(window, 'confirm').mockReturnValue(true)
})
afterEach(() => vi.restoreAllMocks())
async function open(props: { groupId?: string; initialTab?: string } = { groupId: '38' }) {
  const wrapper = mount(ClientAccessManager, { props, global: { stubs: {
    RouterLink: { template: '<a><slot /></a>' },
    BaseDialog: { name: 'BaseDialog', props: ['show', 'title'], template: '<div v-if="show" role="dialog"><h2>{{ title }}</h2><slot /></div>' }
  } } })
  await flushPromises()
  return wrapper
}
const selectMode = async (wrapper: Awaited<ReturnType<typeof open>>, mode: string) => wrapper.get('[data-testid="policy-mode"]').setValue(mode)
const submit = async (wrapper: Awaited<ReturnType<typeof open>>) => { await wrapper.get('form').trigger('submit'); await flushPromises() }

describe('ClientAccessManager', () => {
  it('opens the current group directly without a group picker or registry', async () => {
    const wrapper = await open()
    expect(wrapper.get('h1').text()).toBe('客户端访问限制 · 开发组')
    expect(wrapper.text()).toContain('当前 API Key 所属分组仅允许使用：Codex 官方客户端')
    expect(wrapper.text()).not.toContain('选择分组')
    expect(wrapper.find('input[type="search"]').exists()).toBe(false)
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.get('[data-testid="save-client-access"]').text()).toBe('保存当前分组配置')
    expect(wrapper.get('[data-testid="save-client-access"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('返回分组列表')
    expect(wrapper.text()).toContain('管理客户端')
  })
  it('saves a multi-select deny policy without changing clients or other groups', async () => {
    const wrapper = await open()
    await selectMode(wrapper, 'deny')
    await wrapper.get('[data-client-id="deepseek_harness"]').setValue(true)
    expect(wrapper.text()).toContain('禁止使用：Codex 官方客户端、DeepSeek Harness')
    await submit(wrapper)
    expect(mocks.save).toHaveBeenCalledTimes(1)
    expect(mocks.save.mock.calls[0][0]).toEqual({ ...config(), groups: { '38': { mode: 'deny', client_ids: ['codex', 'deepseek_harness'] } } })
    expect(mocks.get).toHaveBeenCalledTimes(2)
    expect(wrapper.vm.dirty).toBe(false)
  })
  it('preserves an independent restriction on another group', async () => {
    const initial = config()
    initial.groups['99'] = { mode: 'deny', client_ids: ['deepseek_harness'] }
    mocks.get.mockResolvedValueOnce(initial)
    const wrapper = await open()
    await selectMode(wrapper, 'deny')
    await submit(wrapper)
    expect(mocks.save.mock.calls[0][0].groups['99']).toEqual({ mode: 'deny', client_ids: ['deepseek_harness'] })
  })
  it('shows a useful empty state before any groups have been created', async () => {
    mocks.groups.mockResolvedValue([])
    mocks.get.mockResolvedValue({ ...config(), groups: {} })
    const wrapper = await open({})
    expect(wrapper.text()).toContain('请先创建分组')
    expect(wrapper.text()).toContain('暂无分组限制策略')
    expect(wrapper.get('[data-testid="add-policy"]').attributes('disabled')).toBeDefined()
  })
  it('does not claim a new policy is dirty until it has been changed', async () => {
    const wrapper = await open({})
    await wrapper.get('[data-testid="add-policy"]').trigger('click')
    expect(wrapper.vm.dirty).toBe(false)
    expect(wrapper.get('[data-testid="policy-mode"]').findAll('option').map(option => option.attributes('value'))).toEqual(['allow', 'deny'])
    await wrapper.get('[data-testid="new-policy-group"]').setValue('99')
    expect(wrapper.vm.dirty).toBe(true)
  })
  it('shows a global policy list and a clear add action without an implicit editor', async () => {
    const wrapper = await open({})
    expect(wrapper.get('[data-testid="add-policy"]').text()).toBe('新增分组策略')
    expect(wrapper.findAll('[data-policy-id]')).toHaveLength(1)
    expect(wrapper.text()).toContain('开发组')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('保存全部')
  })
  it('creates a policy for an unconfigured group, preserving existing policies', async () => {
    const wrapper = await open({})
    await wrapper.get('[data-testid="add-policy"]').trigger('click')
    const picker = wrapper.get('[data-testid="new-policy-group"]')
    expect(picker.findAll('option').map(option => option.attributes('value'))).toEqual(['', '99'])
    await picker.setValue('99')
    await selectMode(wrapper, 'deny')
    await wrapper.get('[data-client-id="deepseek_harness"]').setValue(true)
    await submit(wrapper)
    expect(mocks.save.mock.calls[0][0].groups).toEqual({ ...config().groups, '99': { mode: 'deny', client_ids: ['deepseek_harness'] } })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-policy-id]')).toHaveLength(2)
    expect(wrapper.get('[data-testid="add-policy"]').attributes('disabled')).toBeDefined()
  })
  it('edits an existing policy without allowing a group switch', async () => {
    const wrapper = await open({})
    await wrapper.get('[data-policy-id="38"]').findAll('button')[0].trigger('click')
    expect(wrapper.find('[data-testid="new-policy-group"]').exists()).toBe(false)
    expect(wrapper.get('[role="dialog"]').text()).toContain('开发组 · ID 38')
    await selectMode(wrapper, 'deny')
    await submit(wrapper)
    expect(mocks.save.mock.calls[0][0].groups['38'].mode).toBe('deny')
  })
  it('removes only the chosen restriction after confirmation', async () => {
    const wrapper = await open({})
    await wrapper.get('[data-policy-id="38"]').findAll('button')[1].trigger('click')
    await flushPromises()
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('分组本身不会删除'))
    expect(mocks.save.mock.calls[0][0].groups).toEqual({})
    expect(wrapper.text()).toContain('暂无分组限制策略')
    expect(wrapper.get('[data-testid="add-policy"]').attributes('disabled')).toBeUndefined()
  })
  it('does not remove restrictions when confirmation is cancelled', async () => {
    vi.mocked(window.confirm).mockReturnValue(false)
    const wrapper = await open({})
    await wrapper.get('[data-policy-id="38"]').findAll('button')[1].trigger('click')
    expect(mocks.save).not.toHaveBeenCalled()
  })
  it('rejects an empty client selection without sending a write', async () => {
    const wrapper = await open()
    await wrapper.get('[data-client-id="codex"]').setValue(false)
    await submit(wrapper)
    expect(mocks.save).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('至少选择一个')
  })
  it('restores unrestricted mode without deleting the group or other config', async () => {
    const wrapper = await open()
    await selectMode(wrapper, 'off')
    await submit(wrapper)
    expect(mocks.save.mock.calls[0][0]).toEqual({ ...config(), groups: {} })
  })
  it('keeps edits on a concurrency conflict', async () => {
    mocks.save.mockRejectedValue({ status: 409 })
    const wrapper = await open()
    await selectMode(wrapper, 'deny')
    await submit(wrapper)
    expect(wrapper.get('[role="alert"]').text()).toContain('其他管理员')
    expect(wrapper.get('[data-testid="policy-mode"]').element).toHaveProperty('value', 'deny')
    expect(mocks.get).toHaveBeenCalledTimes(1)
    expect(wrapper.vm.dirty).toBe(true)
  })
  it.each(['999', '038', '', 'invalid'])('does not fall back to another group for invalid ID %s', async groupId => {
    const wrapper = await open({ groupId })
    expect(wrapper.get('[role="alert"]').text()).toContain('不存在或不可访问')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('[data-testid="add-policy"]').exists()).toBe(false)
    expect(mocks.save).not.toHaveBeenCalled()
  })
  it('shows load failures without an editable default configuration', async () => {
    mocks.get.mockRejectedValue(new Error('offline'))
    const wrapper = await open()
    expect(wrapper.get('[role="alert"]').text()).toContain('加载失败')
    expect(wrapper.find('form').exists()).toBe(false)
  })
  it('adds custom clients separately while preserving all saved group policies', async () => {
    const wrapper = await open({ initialTab: 'clients' })
    const codex = wrapper.findAll('details')[0]
    expect(codex.find('input[type="checkbox"]').attributes('disabled')).toBeDefined()
    expect(codex.find('button').attributes('disabled')).toBeDefined()
    await wrapper.findAll('section.card')[0].get('button').trigger('click')
    const inputs = wrapper.findAll('details')[2].findAll('input:not([type="checkbox"])')
    await inputs[0].setValue('company_tool')
    await inputs[1].setValue('公司工具')
    await inputs[2].setValue('X-Company-Client')
    await inputs[3].setValue('company/')
    expect(wrapper.get('[data-testid="save-clients"]').text()).toBe('保存客户端规则')
    expect(wrapper.find('[data-testid="policy-mode"]').exists()).toBe(false)
    await submit(wrapper)
    expect(mocks.save.mock.calls[0][0].clients[2]).toMatchObject({ id: 'company_tool', name: '公司工具', rules: [{ header: 'X-Company-Client', operator: 'prefix', value: 'company/' }] })
    expect(mocks.save.mock.calls[0][0].groups).toEqual(config().groups)
    expect(wrapper.vm.dirty).toBe(false)
  })
  it('confirms before discarding registry edits on a tab switch', async () => {
    const wrapper = await open({ initialTab: 'clients' })
    await wrapper.findAll('details')[0].findAll('input')[1].setValue('重命名')
    vi.mocked(window.confirm).mockReturnValue(false)
    await wrapper.get('nav').findAll('button')[0].trigger('click')
    expect(wrapper.find('details').exists()).toBe(true)
    expect(wrapper.vm.dirty).toBe(true)
    expect(mocks.save).not.toHaveBeenCalled()
  })
  it('locks further writes when saving succeeded but revision readback failed', async () => {
    const wrapper = await open()
    mocks.get.mockRejectedValueOnce(new Error('readback offline'))
    await selectMode(wrapper, 'deny')
    await submit(wrapper)
    expect(wrapper.get('[role="alert"]').text()).toContain('配置已提交')
    expect(wrapper.find('fieldset').attributes('disabled')).toBeDefined()
    await submit(wrapper)
    expect(mocks.save).toHaveBeenCalledTimes(1)
  })
  it('keeps the readback failure visible after closing the global editor', async () => {
    const wrapper = await open({})
    await wrapper.get('[data-policy-id="38"]').findAll('button')[0].trigger('click')
    await selectMode(wrapper, 'deny')
    mocks.get.mockRejectedValueOnce(new Error('readback offline'))
    await submit(wrapper)
    wrapper.findComponent({ name: 'BaseDialog' }).vm.$emit('close')
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toContain('配置已提交')
    expect(wrapper.get('[data-testid="add-policy"]').attributes('disabled')).toBeDefined()
  })
  it('uses the fresh revision for a subsequent save', async () => {
    const wrapper = await open()
    await selectMode(wrapper, 'deny')
    await submit(wrapper)
    await selectMode(wrapper, 'allow')
    await submit(wrapper)
    expect(mocks.save.mock.calls[1][0].revision).toBe('r2')
  })
  it('uses configuration names and all references, not hardcoded group text', () => {
    const cfg = config()
    cfg.clients[0].name = '新名称'
    expect(policyPreview(cfg.groups['38'], cfg.clients)).toBe('新名称')
    expect(referencingGroups(cfg, 'codex')).toEqual(['38'])
    expect(referencingGroups(cfg, 'deepseek_harness')).toEqual([])
  })
})
