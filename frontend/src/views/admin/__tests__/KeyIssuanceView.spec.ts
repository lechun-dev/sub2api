import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

import type { AdminUser } from '@/types'
import KeyIssuanceView from '../KeyIssuanceView.vue'

const {
  listUsers,
  listAdminAPIKeys,
  getAllGroups,
  getAllIncludingInactive,
  getPublicSettings,
  showError,
  copyToClipboard
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  listAdminAPIKeys: vi.fn(),
  getAllGroups: vi.fn(),
  getAllIncludingInactive: vi.fn(),
  getPublicSettings: vi.fn(),
  showError: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { list: listUsers },
    apiKeys: { listAdminAPIKeys },
    groups: { getAll: getAllGroups, getAllIncludingInactive }
  }
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: null,
    showError
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const SelectStub = defineComponent({
  name: 'KeyIssuanceSelectStub',
  props: {
    modelValue: [String, Number, Boolean, Object],
    options: { type: Array, default: () => [] },
    loading: Boolean,
    remote: Boolean
  },
  emits: ['update:modelValue', 'search', 'change'],
  template: `
    <div class="select-stub" :data-remote="remote ? 'true' : 'false'">
      <button type="button" @click="$emit('search', '李群')">search</button>
    </div>
  `
})

const createUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 7,
  username: '李群',
  email: 'liqun@lechun.cc',
  role: 'admin',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-09-20T00:00:00Z',
  updated_at: '2026-09-20T00:00:00Z',
  notes: '',
  ...overrides
})

const mountView = () => mount(KeyIssuanceView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
      DataTable: true,
      Pagination: true,
      SearchInput: true,
      BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
      ConfirmDialog: true,
      Icon: true,
      UseKeyModal: true,
      Select: SelectStub,
      Teleport: true
    }
  }
})

describe('admin KeyIssuanceView recipient search', () => {
  beforeEach(() => {
    listUsers.mockReset()
    listAdminAPIKeys.mockReset()
    getAllGroups.mockReset()
    getAllIncludingInactive.mockReset()
    getPublicSettings.mockReset()
    showError.mockReset()
    copyToClipboard.mockReset()

    listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 1000, pages: 0 })
    listAdminAPIKeys.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    getAllGroups.mockResolvedValue([])
    getAllIncludingInactive.mockResolvedValue([])
    getPublicSettings.mockResolvedValue(null)
    copyToClipboard.mockResolvedValue(true)
  })

  it('searches all active account roles and displays a matching admin user', async () => {
    const wrapper = mountView()
    await flushPromises()
    listUsers.mockResolvedValueOnce({ items: [createUser()], total: 1, page: 1, page_size: 1000, pages: 1 })

    const remoteSelect = wrapper.findAllComponents(SelectStub).find((select) => select.props('remote'))
    expect(remoteSelect).toBeDefined()
    await remoteSelect!.get('button').trigger('click')
    await flushPromises()

    const [, , filters] = listUsers.mock.calls.at(-1) ?? []
    expect(filters).toMatchObject({ status: 'active', search: '李群' })
    expect(filters).not.toHaveProperty('role')
    expect(remoteSelect!.props('options')).toContainEqual({
      value: 7,
      label: '李群 · liqun@lechun.cc'
    })

    wrapper.unmount()
  })
})
