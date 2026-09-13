<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.keyIssuance.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.keyIssuance.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary"
          :title="t('admin.keyIssuance.clear')"
          @click="clearForm"
        >
          <Icon name="refresh" size="md" class="mr-2" />
          {{ t('admin.keyIssuance.clear') }}
        </button>
      </div>

      <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <section class="card p-6">
          <div class="mb-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.keyIssuance.recipient') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.keyIssuance.selectedUser') }}
            </p>
          </div>

          <form class="flex gap-2" @submit.prevent="loadUsers">
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
            class="mt-4 flex items-center justify-between gap-3 rounded-lg border border-primary-200 bg-primary-50 px-4 py-3 dark:border-primary-800 dark:bg-primary-900/20"
          >
            <div class="min-w-0">
              <p class="text-xs font-medium uppercase tracking-wide text-primary-600 dark:text-primary-400">
                {{ t('admin.keyIssuance.selectedUser') }}
              </p>
              <p class="mt-1 truncate font-medium text-gray-900 dark:text-white">
                {{ userLabel(selectedUser) }}
              </p>
            </div>
            <button
              type="button"
              class="shrink-0 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
              :title="t('admin.keyIssuance.clear')"
              @click="clearSelectedUser"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <div class="mt-4 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
            <div v-if="loadingUsers" class="flex items-center justify-center py-10 text-sm text-gray-500 dark:text-gray-400">
              <Icon name="refresh" size="md" class="mr-2 animate-spin" />
              {{ t('common.loading') }}
            </div>
            <div v-else-if="users.length === 0" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.keyIssuance.noUsers') }}
            </div>
            <div v-else class="max-h-80 overflow-y-auto">
              <button
                v-for="user in users"
                :key="user.id"
                type="button"
                class="flex w-full items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 text-left last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800"
                :class="selectedUser?.id === user.id ? 'bg-primary-50 dark:bg-primary-900/20' : ''"
                @click="selectUser(user)"
              >
                <span class="flex min-w-0 items-center gap-3">
                  <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    <Icon name="user" size="sm" />
                  </span>
                  <span class="min-w-0">
                    <span class="block truncate font-medium text-gray-900 dark:text-white">
                      {{ userLabel(user) }}
                    </span>
                    <span class="block truncate text-xs text-gray-500 dark:text-gray-400">
                      {{ user.email }} · #{{ user.id }}
                    </span>
                  </span>
                </span>
                <Icon
                  v-if="selectedUser?.id === user.id"
                  name="check"
                  size="sm"
                  class="shrink-0 text-primary-600 dark:text-primary-400"
                />
              </button>
            </div>
          </div>
        </section>

        <section class="card p-6">
          <div class="mb-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.keyIssuance.keyName') }}
            </h2>
          </div>

          <form class="space-y-4" @submit.prevent="issueKey">
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
                <input
                  v-model.number="form.quota"
                  type="number"
                  min="0"
                  step="0.01"
                  class="input w-full"
                />
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.keyIssuance.quotaHint') }}
                </p>
              </div>
              <div>
                <label class="input-label">{{ t('admin.keyIssuance.expiresInDays') }}</label>
                <input
                  v-model.number="form.expires_in_days"
                  type="number"
                  min="0"
                  step="1"
                  class="input w-full"
                />
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.keyIssuance.expiresNever') }}
                </p>
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

            <button type="submit" class="btn btn-primary w-full" :disabled="issuing || loadingGroups">
              <Icon v-if="issuing" name="refresh" size="md" class="mr-2 animate-spin" />
              <Icon v-else name="key" size="md" class="mr-2" />
              {{ issuing ? t('admin.keyIssuance.issuing') : t('admin.keyIssuance.issue') }}
            </button>
          </form>
        </section>
      </div>

      <section v-if="result" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.keyIssuance.resultTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.keyIssuance.issuedFor') }}: {{ userLabel(selectedUser) }}
              </p>
            </div>
            <span class="inline-flex items-center gap-2 text-sm font-medium text-emerald-600 dark:text-emerald-400">
              <Icon name="check" size="sm" />
              {{ t('admin.keyIssuance.keyCreated') }}
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
              {{ t('admin.keyIssuance.resultWarning') }}
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { getPublicSettings } from '@/api/auth'
import type { AdminGroup, AdminUser, PublicSettings } from '@/types'
import type { IssueAPIKeyResponse } from '@/api/admin/apiKeys'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const searchQuery = ref('')
const users = ref<AdminUser[]>([])
const selectedUser = ref<AdminUser | null>(null)
const groups = ref<AdminGroup[]>([])
const publicSettings = ref<PublicSettings | null>(null)
const loadingUsers = ref(false)
const loadingGroups = ref(false)
const issuing = ref(false)
const result = ref<IssueAPIKeyResponse | null>(null)
const issueOperationKey = ref<string | null>(null)

const form = reactive({
  name: '',
  group_id: null as number | null,
  quota: 0,
  expires_in_days: 0,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0
})

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
    issueOperationKey.value = null
  }
  selectedUser.value = user
}

function clearSelectedUser(): void {
  selectedUser.value = null
  result.value = null
  issueOperationKey.value = null
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
    appStore.showSuccess(t('admin.keyIssuance.keyCreated'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.keyIssuance.failedToIssue')))
  } finally {
    issuing.value = false
  }
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
  selectedUser.value = null
  result.value = null
  issueOperationKey.value = null
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
    if (!issuing.value) {
      issueOperationKey.value = null
    }
  },
  { deep: true }
)

onMounted(() => {
  void Promise.all([loadUsers(), loadGroups(), loadSettings()])
})
</script>
