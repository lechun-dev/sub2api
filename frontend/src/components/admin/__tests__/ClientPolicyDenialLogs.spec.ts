import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ClientPolicyDenialLogs from '../ClientPolicyDenialLogs.vue'
import ClientPolicyDates from '../ClientPolicyDates.vue'
import GroupClientPolicyEditor from '../GroupClientPolicyEditor.vue'
import messages from '@/i18n/locales/zh/admin/clientAccess'
const mocks = vi.hoisted(() => ({ logs: vi.fn() }))
vi.mock('@/api/admin/clientAccess', async importOriginal => ({ ...(await importOriginal<object>()), getClientPolicyDenials: mocks.logs }))
vi.mock('vue-i18n', async importOriginal => ({ ...(await importOriginal<object>()), useI18n: () => ({ t: (key: string, params: Record<string, string | number> = {}) => {
  let text: string = messages.clientAccess[key.replace('admin.clientAccess.', '') as keyof typeof messages.clientAccess] ?? key
  for (const [name, value] of Object.entries(params)) text = text.replace(`{${name}}`, String(value))
  return text
} }) }))
enableAutoUnmount(afterEach)
const fixture = { id: 1, created_at: '2026-10-14T01:00:00Z', group_id: 38, user_id: 72, username: '张三', api_key_id: 41, client_ip: '192.0.2.1', request_id: 'request-1', request_path: '/responses', snapshot: {
  code: 'client_not_allowed', mode: 'allow', detected_clients: [{ id: 'deepseek_harness', name: '旧版客户端名' }], detected_count: 1,
  selected_clients: [{ id: 'codex', name: '拦截时的 Codex' }], selected_count: 1
} }
function open() {
  return mount(ClientPolicyDenialLogs, { props: { groupId: '38', groupName: '开发组' }, global: { stubs: { BaseDialog: { props: ['show', 'title'], template: '<div><h2>{{ title }}</h2><slot /></div>' } } } })
}
beforeEach(() => { vi.resetAllMocks(); mocks.logs.mockResolvedValue({ items: [fixture], total: 21, page: 1, page_size: 20, pages: 2 }) })
describe('policy dates and denial logs', () => {
  it('shows server dates and never fabricates historical dates', () => {
    const wrapper = mount(ClientPolicyDates, { props: { policy: { mode: 'allow', client_ids: ['codex'], updated_at: '2026-10-14T01:00:00Z' } } })
    expect(wrapper.text()).toContain('历史未记录')
    expect(wrapper.text()).toContain('2026')
  })
  it('retains date metadata on mode edits so returning to the original mode stays unchanged', async () => {
    const policy = { mode: 'allow' as const, client_ids: ['codex'], created_at: '2026-10-14T01:00:00Z' }
    const wrapper = mount(GroupClientPolicyEditor, { props: { modelValue: policy, clients: [], saving: false, dirty: false, canSave: true } })
    await wrapper.get('select').setValue('deny')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ ...policy, mode: 'deny' }])
  })
  it('queries exactly the selected group and displays the historical snapshot', async () => {
    const wrapper = open(); await flushPromises()
    const params = mocks.logs.mock.calls[0][0]
    expect(params.group_id).toBe('38')
    expect(Date.parse(params.end_time) - Date.parse(params.start_time)).toBe(86400000)
    expect(wrapper.text()).toContain('开发组（ID 38）')
    expect(wrapper.text()).toContain('旧版客户端名')
    expect(wrapper.text()).toContain('拦截时的 Codex')
    expect(wrapper.text()).toContain('API Key ID: 41')
    expect(wrapper.text()).toContain('用户名: 张三')
    expect(wrapper.text()).not.toContain('用户 ID')
    expect(wrapper.text()).not.toContain('72')
    expect(wrapper.text()).toContain('当前客户端不在允许列表内')
  })
  it('keeps a fixed date range while paging and resets it when selecting seven days', async () => {
    const wrapper = open(); await flushPromises()
    const first = mocks.logs.mock.calls[0][0]
    await wrapper.findAll('button').find(b => b.text() === '下一页')!.trigger('click'); await flushPromises()
    expect(mocks.logs.mock.calls[1][0]).toEqual({ ...first, page: 2 })
    await wrapper.get('[data-testid="log-range"]').setValue('7'); await flushPromises()
    const last = mocks.logs.mock.calls[2][0]
    expect(last.page).toBe(1)
    expect(Date.parse(last.end_time) - Date.parse(last.start_time)).toBe(7 * 86400000)
  })
  it('distinguishes disabled monitoring from failed routes and empty results', async () => {
    mocks.logs.mockRejectedValueOnce({ status: 404, code: 'OPS_DISABLED' })
    const wrapper = open(); await flushPromises()
    expect(wrapper.text()).toContain('运维监控未开启')
    expect(wrapper.text()).not.toContain('没有记录')
    mocks.logs.mockRejectedValueOnce({ status: 404 })
    await wrapper.get('button').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('加载失败')
    mocks.logs.mockResolvedValueOnce({ items: [], total: 0 })
    await wrapper.get('button').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('此时间范围没有记录')
  })
  it('shows an unavailable username without falling back to user IDs', async () => {
    mocks.logs.mockResolvedValueOnce({ items: [{ ...fixture, username: '   ' }], total: 1 })
    const wrapper = open(); await flushPromises()
    expect(wrapper.text()).toContain('用户名: 用户不可用')
    expect(wrapper.text()).not.toContain('72')
  })
  it('isolates missing snapshots without breaking the list', async () => {
    mocks.logs.mockResolvedValueOnce({ items: [{ ...fixture, snapshot: undefined }], total: 1 })
    const wrapper = open(); await flushPromises()
    expect(wrapper.text()).toContain('历史策略快照不可用')
    expect(wrapper.text()).toContain('request-1')
  })
  it('aborts old requests and ignores late responses when switching groups', async () => {
    let resolveOld!: (value: unknown) => void
    mocks.logs.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const wrapper = open(); await flushPromises()
    const signal = mocks.logs.mock.calls[0][1] as AbortSignal
    mocks.logs.mockResolvedValueOnce({ items: [{ ...fixture, group_id: 99, request_id: 'new-request' }], total: 1 })
    await wrapper.setProps({ groupId: '99', groupName: '另一组' }); await flushPromises()
    expect(signal.aborted).toBe(true)
    resolveOld({ items: [fixture], total: 1 }); await flushPromises()
    expect(wrapper.text()).toContain('new-request')
    expect(wrapper.text()).not.toContain('request-1')
    expect(mocks.logs.mock.calls[1][0].group_id).toBe('99')
  })
})
