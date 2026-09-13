<template>
  <AppLayout>
    <div class="space-y-6">
      <TablePageLayout>
        <template #filters>
          <div class="space-y-3">
            <div class="flex flex-wrap items-center gap-3">
              <form class="flex w-full max-w-md gap-2" @submit.prevent="loadUsers">
                <div class="relative flex-1">
                  <Icon
                    name="search"
                    size="md"
                    class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
                  />
                  <input
                    v-model="searchQuery"
                    type="search"
                    class="input w-full pl-10"
                    :placeholder="t('admin.keyIssuance.searchUsers')"
                  />
                </div>
                <button
                  type="submit"
                  class="btn btn-secondary"
                  :disabled="loadingUsers"
                  :title="t('common.search')"
                >
                  <Icon name="search" size="md" />
                </button>
              </form>

              <div
                v-if="selectedUser"
                class="flex min-w-64 flex-1 items-center justify-between gap-3 rounded-lg border border-primary-200 bg-primary-50 px-4 py-2.5 dark:border-primary-800 dark:bg-primary-900/20"
              >
                <div class="min-w-0">
                  <p class="text-xs font-medium uppercase tracking-wide text-primary-600 dark:text-primary-400">
                    {{ t('admin.keyIssuance.selectedUser') }}
                  </p>
                  <p class="truncate font-medium text-gray-900 dark:text-white">
                    {{ userLabel(selectedUser) }}
                    <span class="ml-1 text-xs font-normal text-gray-500 dark:text-gray-400">#{{ selectedUser.id }}</span>
                  </p>
                </div>
                <button
                  type="button"
                  class="shrink-0 rounded-lg p-1 text-gray-400 transition-colors hover:bg-white hover:text-gray-700 dark:hover:bg-dark-800 dark:hover:text-gray-200"
                  :title="t('admin.keyIssuance.clearSelection')"
                  @click="clearSelectedUser"
                >
                  <Icon name="x" size="md" />
                </button>
              </div>
            </div>

            <div v-if="!selectedUser" class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
              <div v-if="loadingUsers" class="flex items-center justify-center py-6 text-sm text-gray-500 dark:text-gray-400">
                <Icon name="refresh" size="md" class="mr-2 animate-spin" />
                {{ t('common.loading') }}
              </div>
              <div v-else-if="users.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.keyIssuance.noUsers') }}
              </div>
              <div v-else class="flex max-h-44 flex-wrap gap-2 overflow-y-auto bg-white p-3 dark:bg-dark-900">
                <button
                  v-for="user in users"
                  :key="user.id"
                  type="button"
                  class="flex min-w-52 flex-1 items-center gap-3 rounded-lg border border-gray-200 px-3 py-2 text-left transition-colors hover:border-primary-300 hover:bg-primary-50 dark:border-dark-700 dark:hover:border-primary-700 dark:hover:bg-primary-900/20"
                  @click="selectUser(user)"
                >
                  <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    <Icon name="user" size="sm" />
                  </span>
                  <span class="min-w-0">
                    <span class="block truncate font-medium text-gray-900 dark:text-white">{{ userLabel(user) }}</span>
                    <span class="block truncate text-xs text-gray-500 dark:text-gray-400">{{ user.email }} · #{{ user.id }}</span>
                  </span>
                </button>
              </div>
            </div>
          </div>
        </template>

        <template #actions>
          <div class="flex flex-wrap justify-end gap-3">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loadingUsers || loadingExistingKeys"
              :title="t('common.refresh')"
              @click="refreshPage"
            >
              <Icon name="refresh" size="md" :class="(loadingUsers || loadingExistingKeys) ? 'animate-spin' : ''" />
            </button>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="!selectedUser"
              @click="clearForm"
            >
              <Icon name="refresh" size="md" class="mr-2" />
              {{ t('admin.keyIssuance.clear') }}
            </button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="!selectedUser"
              @click="showCreateDialog = true"
            >
              <Icon name="plus" size="md" class="mr-2" />
              {{ t('admin.keyIssuance.createNewKey') }}
            </button>
          </div>
        </template>

        <template #table>
          <div v-if="selectedUser" class="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.keyIssuance.existingKeys') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.keyIssuance.existingKeysHint') }}</p>
            </div>
            <span class="rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
              {{ existingKeys.length }}
            </span>
          </div>

          <div v-if="!selectedUser" class="flex h-full min-h-72 flex-col items-center justify-center px-6 text-center">
            <Icon name="key" size="xl" class="mb-4 text-gray-300 dark:text-dark-500" />
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.keyIssuance.chooseUserTitle') }}</h2>
            <p class="mt-1 max-w-md text-sm text-gray-500 dark:text-gray-400">{{ t('admin.keyIssuance.chooseUserDescription') }}</p>
          </div>

          <DataTable
            v-else
            :columns="columns"
            :data="existingKeys"
            :loading="loadingExistingKeys"
            default-sort-key="created_at"
            default-sort-order="desc"
          >
            <template #cell-name="{ value, row }">
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ value }}</p>
                <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">#{{ row.id }}</p>
              </div>
            </template>

            <template #cell-key="{ value, row }">
              <div class="flex items-center gap-2">
                <code class="code text-xs">{{ maskApiKey(value) }}</code>
                <button
                  type="button"
                  class="rounded-lg p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                  :class="copiedKeyId === row.id ? 'text-emerald-500' : ''"
                  :title="copiedKeyId === row.id ? t('admin.keyIssuance.copied') : t('keys.copyToClipboard')"
                  @click="copyExistingKey(row)"
                >
                  <Icon :name="copiedKeyId === row.id ? 'check' : 'clipboard'" size="sm" />
                </button>
              </div>
            </template>

            <template #cell-group="{ row }">
              <span class="text-sm text-gray-600 dark:text-gray-300">
                {{ row.group?.name || t('admin.keyIssuance.noGroup') }}
              </span>
            </template>

            <template #cell-current_concurrency="{ value }">
              <span
                :class="[
                  'inline-flex min-w-8 items-center justify-center rounded px-2 py-1 text-sm font-semibold tabular-nums',
                  (value ?? 0) > 0
                    ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-300 dark:ring-emerald-800'
                    : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400'
                ]"
              >
                {{ value ?? 0 }}
              </span>
            </template>

            <template #cell-usage="{ row }">
              <div class="text-sm text-gray-600 dark:text-gray-300">
                <span>${{ row.quota_used?.toFixed(4) || '0.0000' }}</span>
                <span v-if="row.quota > 0" class="text-gray-400 dark:text-gray-500"> / ${{ row.quota.toFixed(2) }}</span>
              </div>
              <div v-if="row.quota > 0" class="mt-1 h-1.5 w-28 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="h-full rounded-full bg-primary-500 transition-all"
                  :style="{ width: `${Math.min((row.quota_used / row.quota) * 100, 100)}%` }"
                />
              </div>
            </template>

            <template #cell-expires_at="{ value }">
              <span class="text-sm text-gray-500 dark:text-dark-400">
                {{ value ? formatDateTime(value) : t('admin.keyIssuance.expiresNever') }}
              </span>
            </template>

            <template #cell-status="{ value }">
              <span :class="['badge', keyStatusClass(value)]">{{ keyStatusLabel(value) }}</span>
            </template>

            <template #cell-created_at="{ value }">
              <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <button
                type="button"
                class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-primary-600 transition-colors hover:bg-primary-50 disabled:cursor-not-allowed disabled:text-gray-400 disabled:hover:bg-transparent dark:text-primary-400 dark:hover:bg-primary-900/20 dark:disabled:text-dark-500 dark:disabled:hover:bg-transparent"
                :disabled="row.status !== 'active'"
                :title="row.status === 'active' ? t('admin.keyIssuance.directIssue') : t('admin.keyIssuance.keyUnavailable')"
                @click="deliverExistingKey(row)"
              >
                <Icon name="key" size="sm" />
                {{ t('admin.keyIssuance.directIssue') }}
              </button>
            </template>

            <template #empty>
              <div class="flex flex-col items-center py-10">
                <Icon name="key" size="xl" class="mb-3 text-gray-300 dark:text-dark-500" />
                <p class="font-medium text-gray-900 dark:text-white">{{ t('admin.keyIssuance.noExistingKeys') }}</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.keyIssuance.createNewKeyHint') }}</p>
              </div>
            </template>
          </DataTable>
        </template>
      </TablePageLayout>

      <section v-if="result" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ resultSource === 'existing' ? t('admin.keyIssuance.existingResultTitle') : t('admin.keyIssuance.resultTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.keyIssuance.issuedFor') }}: {{ userLabel(selectedUser) }}
              </p>
            </div>
            <span class="inline-flex items-center gap-2 text-sm font-medium text-emerald-600 dark:text-emerald-400">
              <Icon name="check" size="sm" />
              {{ resultSource === 'existing' ? t('admin.keyIssuance.directIssued') : t('admin.keyIssuance.keyCreated') }}
            </span>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-6 p-6 xl:grid-cols-2">
          <div>
            <div class="mb-2 flex items-center justify-between gap-3">
              <label class="input-label mb-0">{{ t('admin.keyIssuance.plaintextKey') }}</label>
              <button type="button" class="btn btn-secondary btn-sm" @click="copyKey">
                <Icon name="copy" size="sm" class="mr-2" />
                {{ t('admin.keyIssuance.copyKey') }}
              </button>
            </div>
            <textarea
              :value="result.plaintext_key"
              readonly
              rows="3"
              class="w-full resize-none rounded-lg border border-amber-300 bg-amber-50 p-3 font-mono text-sm text-gray-900 outline-none dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-100"
            />
            <p class="mt-2 text-sm text-amber-700 dark:text-amber-300">
              {{ resultSource === 'existing' ? t('admin.keyIssuance.existingResultWarning') : t('admin.keyIssuance.resultWarning') }}
            </p>
          </div>

          <div>
            <div class="mb-2 flex items-center justify-between gap-3">
              <label class="input-label mb-0">{{ t('admin.keyIssuance.instructionsTitle') }}</label>
              <button type="button" class="btn btn-secondary btn-sm" @click="copyInstructions">
                <Icon name="copy" size="sm" class="mr-2" />
                {{ t('admin.keyIssuance.copyInstructions') }}
              </button>
            </div>
            <textarea
              :value="instructionsText"
              readonly
              rows="7"
              class="w-full resize-none rounded-lg border border-gray-200 bg-gray-50 p-3 font-mono text-xs leading-5 text-gray-700 outline-none dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
            />
          </div>
        </div>
      </section>
    </div>

    <BaseDialog
      :show="showCreateDialog"
      :title="t('admin.keyIssuance.createNewKey')"
      width="wide"
      @close="showCreateDialog = false"
    >
      <form id="admin-key-form" class="space-y-5" @submit.prevent="issueKey">
        <div class="rounded-lg border border-primary-100 bg-primary-50/60 px-4 py-3 dark:border-primary-900 dark:bg-primary-900/20">
          <p class="text-xs font-medium uppercase tracking-wide text-primary-600 dark:text-primary-400">{{ t('admin.keyIssuance.selectedUser') }}</p>
          <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ userLabel(selectedUser) }}</p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.keyIssuance.keyName') }}</label>
          <input
            v-model="form.name"
            type="text"
            class="input w-full"
            :placeholder="t('admin.keyIssuance.keyNamePlaceholder')"
            maxlength="100"
          />
        </div>

        <div>
          <label class="input-label">{{ t('admin.keyIssuance.group') }}</label>
          <Select
            v-model="form.group_id"
            :options="groupOptions"
            :loading="loadingGroups"
            :placeholder="t('admin.keyIssuance.group')"
            searchable
          />
        </div>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.keyIssuance.quota') }}</label>
            <input v-model.number="form.quota" type="number" min="0" step="0.01" class="input w-full" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.keyIssuance.quotaHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.keyIssuance.expiresInDays') }}</label>
            <input v-model.number="form.expires_in_days" type="number" min="0" step="1" class="input w-full" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.keyIssuance.expiresNever') }}</p>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <label class="input-label">{{ t('admin.keyIssuance.rateLimit5h') }}</label>
            <input v-model.number="form.rate_limit_5h" type="number" min="0" step="0.01" class="input w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.keyIssuance.rateLimit1d') }}</label>
            <input v-model.number="form.rate_limit_1d" type="number" min="0" step="0.01" class="input w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.keyIssuance.rateLimit7d') }}</label>
            <input v-model.number="form.rate_limit_7d" type="number" min="0" step="0.01" class="input w-full" />
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="showCreateDialog = false">{{ t('common.cancel') }}</button>
          <button type="submit" form="admin-key-form" class="btn btn-primary" :disabled="issuing || loadingGroups">
            <Icon v-if="issuing" name="refresh" size="md" class="mr-2 animate-spin" />
            <Icon v-else name="key" size="md" class="mr-2" />
            {{ issuing ? t('admin.keyIssuance.issuing') : t('admin.keyIssuance.issue') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { getPublicSettings } from '@/api/auth'
import type { AdminGroup, AdminUser, ApiKey, PublicSettings } from '@/types'
import type { IssueAPIKeyResponse } from '@/api/admin/apiKeys'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const searchQuery = ref('')
const users = ref<AdminUser[]>([])
const selectedUser = ref<AdminUser | null>(null)
const groups = ref<AdminGroup[]>([])
const publicSettings = ref<PublicSettings | null>(null)
const existingKeys = ref<ApiKey[]>([])
const loadingUsers = ref(false)
const loadingGroups = ref(false)
const loadingExistingKeys = ref(false)
const issuing = ref(false)
const showCreateDialog = ref(false)
const result = ref<IssueAPIKeyResponse | null>(null)
const resultSource = ref<'existing' | 'new' | null>(null)
const issueOperationKey = ref<string | null>(null)
const copiedKeyId = ref<number | null>(null)
let existingKeysRequestId = 0

const form = reactive({
  name: '',
  group_id: null as number | null,
  quota: 0,
  expires_in_days: 0,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0
})

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name'), sortable: true },
  { key: 'key', label: t('keys.apiKey'), sortable: false },
  { key: 'group', label: t('keys.group'), sortable: false },
  { key: 'current_concurrency', label: t('keys.currentConcurrency'), sortable: true },
  { key: 'usage', label: t('keys.usage'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: true },
  { key: 'status', label: t('common.status'), sortable: true },
  { key: 'created_at', label: t('keys.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

const groupOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.keyIssuance.noGroup') },
  ...groups.value.map((group) => ({
    value: group.id,
    label: `${group.name} (#${group.id})`
  }))
])

const apiBaseURL = computed(() => {
  const configured = publicSettings.value?.api_base_url?.trim()
  return (configured || window.location.origin).replace(/\/+$/, '')
})

const instructionsText = computed(() => {
  const key = result.value?.plaintext_key || ''
  const apiURL = `${apiBaseURL.value}/v1`
  return [
    `${t('admin.keyIssuance.baseUrl')}: ${apiURL}`,
    `${t('admin.keyIssuance.authHeader')}: Bearer ${key}`,
    '',
    `${t('admin.keyIssuance.example')}:`,
    `curl ${apiURL}/models -H "Authorization: Bearer ${key}"`
  ].join('\n')
})

function userLabel(user: AdminUser | null): string {
  if (!user) return '-'
  return user.username?.trim() || user.email
}

function getIssueOperationKey(): string {
  if (issueOperationKey.value) return issueOperationKey.value
  try {
    issueOperationKey.value = globalThis.crypto.randomUUID()
  } catch {
    issueOperationKey.value = `admin-key-${Date.now()}-${Math.random().toString(36).slice(2)}`
  }
  return issueOperationKey.value
}

function normalizeNumber(value: number, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : fallback
}

function keyStatusClass(status: ApiKey['status']): string {
  if (status === 'active') return 'badge-success'
  if (status === 'quota_exhausted') return 'badge-warning'
  return 'badge-danger'
}

function keyStatusLabel(status: ApiKey['status']): string {
  const labels: Record<ApiKey['status'], string> = {
    active: t('admin.keyIssuance.statusActive'),
    inactive: t('admin.keyIssuance.statusInactive'),
    quota_exhausted: t('admin.keyIssuance.statusQuotaExhausted'),
    expired: t('admin.keyIssuance.statusExpired')
  }
  return labels[status]
}

async function loadUsers(): Promise<void> {
  loadingUsers.value = true
  try {
    const response = await adminAPI.users.list(1, 20, {
      status: 'active',
      role: 'user',
      search: searchQuery.value.trim() || undefined,
      sort_by: 'email',
      sort_order: 'asc'
    })
    users.value = response.items
  } catch (error) {
    users.value = []
    appStore.showError(extractApiErrorMessage(error, t('admin.keyIssuance.failedToLoadUsers')))
  } finally {
    loadingUsers.value = false
  }
}

async function loadExistingKeys(userId: number): Promise<void> {
  const requestId = ++existingKeysRequestId
  loadingExistingKeys.value = true
  try {
    const response = await adminAPI.users.getUserApiKeys(userId)
    if (requestId !== existingKeysRequestId || selectedUser.value?.id !== userId) return
    existingKeys.value = response.items || []
  } catch (error) {
    if (requestId !== existingKeysRequestId || selectedUser.value?.id !== userId) return
    existingKeys.value = []
    appStore.showError(extractApiErrorMessage(error, t('admin.keyIssuance.failedToLoadExistingKeys')))
  } finally {
    if (requestId === existingKeysRequestId) loadingExistingKeys.value = false
  }
}

async function loadGroups(): Promise<void> {
  loadingGroups.value = true
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch (error) {
    groups.value = []
    appStore.showError(extractApiErrorMessage(error, t('admin.keyIssuance.failedToLoadGroups')))
  } finally {
    loadingGroups.value = false
  }
}

async function loadSettings(): Promise<void> {
  if (appStore.cachedPublicSettings) {
    publicSettings.value = appStore.cachedPublicSettings
    return
  }
  try {
    publicSettings.value = await getPublicSettings()
  } catch {
    publicSettings.value = null
  }
}

function selectUser(user: AdminUser): void {
  if (user.status !== 'active') {
    appStore.showError(t('admin.keyIssuance.userInactive'))
    return
  }

  if (selectedUser.value?.id !== user.id) {
    result.value = null
    resultSource.value = null
    issueOperationKey.value = null
    existingKeysRequestId += 1
    existingKeys.value = []
    copiedKeyId.value = null
    selectedUser.value = user
    void loadExistingKeys(user.id)
    return
  }

  selectedUser.value = user
}

function clearSelectedUser(): void {
  existingKeysRequestId += 1
  selectedUser.value = null
  existingKeys.value = []
  loadingExistingKeys.value = false
  result.value = null
  resultSource.value = null
  issueOperationKey.value = null
  copiedKeyId.value = null
}

async function refreshPage(): Promise<void> {
  await loadUsers()
  if (selectedUser.value) await loadExistingKeys(selectedUser.value.id)
}

async function issueKey(): Promise<void> {
  if (!selectedUser.value) {
    appStore.showError(t('admin.keyIssuance.userRequired'))
    return
  }

  const name = form.name.trim()
  if (!name) {
    appStore.showError(t('admin.keyIssuance.nameRequired'))
    return
  }

  issuing.value = true
  try {
    result.value = await adminAPI.apiKeys.issueAPIKey(
      {
        user_id: selectedUser.value.id,
        name,
        group_id: form.group_id,
        quota: normalizeNumber(form.quota),
        expires_in_days: normalizeNumber(form.expires_in_days) > 0
          ? Math.floor(normalizeNumber(form.expires_in_days))
          : null,
        rate_limit_5h: normalizeNumber(form.rate_limit_5h),
        rate_limit_1d: normalizeNumber(form.rate_limit_1d),
        rate_limit_7d: normalizeNumber(form.rate_limit_7d)
      },
      getIssueOperationKey()
    )
    resultSource.value = 'new'
    showCreateDialog.value = false
    appStore.showSuccess(t('admin.keyIssuance.keyCreated'))
    await loadExistingKeys(selectedUser.value.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.keyIssuance.failedToIssue')))
  } finally {
    issuing.value = false
  }
}

function deliverExistingKey(key: ApiKey): void {
  if (key.status !== 'active' || !key.key) {
    appStore.showError(t('admin.keyIssuance.keyUnavailable'))
    return
  }
  result.value = { api_key: key, plaintext_key: key.key }
  resultSource.value = 'existing'
  issueOperationKey.value = null
  appStore.showSuccess(t('admin.keyIssuance.existingKeySelected'))
}

async function copyExistingKey(key: ApiKey): Promise<void> {
  if (!key.key) {
    appStore.showError(t('admin.keyIssuance.keyUnavailable'))
    return
  }
  copiedKeyId.value = key.id
  await copyToClipboard(key.key, t('admin.keyIssuance.copied'))
  window.setTimeout(() => {
    if (copiedKeyId.value === key.id) copiedKeyId.value = null
  }, 1500)
}

async function copyKey(): Promise<void> {
  if (result.value?.plaintext_key) {
    await copyToClipboard(result.value.plaintext_key, t('admin.keyIssuance.copied'))
  }
}

async function copyInstructions(): Promise<void> {
  await copyToClipboard(instructionsText.value, t('admin.keyIssuance.copied'))
}

function clearForm(): void {
  clearSelectedUser()
  searchQuery.value = ''
  form.name = ''
  form.group_id = null
  form.quota = 0
  form.expires_in_days = 0
  form.rate_limit_5h = 0
  form.rate_limit_1d = 0
  form.rate_limit_7d = 0
  void loadUsers()
}

watch(
  [selectedUser, form],
  () => {
    if (!issuing.value) issueOperationKey.value = null
  },
  { deep: true }
)

onMounted(() => {
  void Promise.all([loadUsers(), loadGroups(), loadSettings()])
})
</script>
