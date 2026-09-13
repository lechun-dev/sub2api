<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <SearchInput
            v-model="filterSearch"
            :placeholder="t('admin.keyIssuance.searchKeys')"
            class="w-full sm:w-80"
            @search="onFilterChange"
          />
          <Select
            :model-value="filterGroupId"
            :options="groupFilterOptions"
            class="w-full sm:w-44"
            :aria-label="t('admin.keyIssuance.groupFilter')"
            @update:model-value="onGroupFilterChange"
          />
          <Select
            :model-value="filterStatus"
            :options="statusFilterOptions"
            class="w-full sm:w-40"
            :aria-label="t('admin.keyIssuance.statusFilter')"
            @update:model-value="onStatusFilterChange"
          />
        </div>
      </template>

      <template #actions>
        <div class="flex justify-end gap-3">
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="refreshPage"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button type="button" class="btn btn-primary" @click="openCreateDialog">
            <Icon name="plus" size="md" class="mr-2" />
            {{ t('admin.keyIssuance.createNewKey') }}
          </button>
        </div>
      </template>

      <template #table>
        <div class="flex items-center justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="min-w-0">
            <h2 class="font-semibold text-gray-900 dark:text-white">
              {{ t('admin.keyIssuance.allKeys') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.keyIssuance.allKeysHint') }}
            </p>
          </div>
          <span class="shrink-0 rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
            {{ pagination.total }}
          </span>
        </div>

        <DataTable
          :columns="columns"
          :data="apiKeys"
          :loading="loading"
          :server-side-sort="true"
        >
          <template #cell-user="{ row }">
            <div class="min-w-44">
              <p class="font-medium text-gray-900 dark:text-white">
                {{ keyUserLabel(row) }}
              </p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                {{ row.user?.email || `#${row.user_id}` }}
              </p>
            </div>
          </template>

          <template #cell-name="{ value, row }">
            <div class="min-w-36">
              <p class="font-medium text-gray-900 dark:text-white">{{ value }}</p>
              <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">#{{ row.id }}</p>
            </div>
          </template>

          <template #cell-key="{ value, row }">
            <div class="flex min-w-44 items-center gap-2">
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
            <div class="min-w-32 text-sm text-gray-600 dark:text-gray-300">
              <span>${{ row.quota_used?.toFixed(4) || '0.0000' }}</span>
              <span v-if="row.quota > 0" class="text-gray-400 dark:text-gray-500">
                / ${{ row.quota.toFixed(2) }}
              </span>
              <div v-if="row.quota > 0" class="mt-1.5 h-1.5 w-28 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  :class="[
                    'h-full rounded-full transition-all',
                    row.quota_used >= row.quota
                      ? 'bg-red-500'
                      : row.quota_used >= row.quota * 0.8
                        ? 'bg-yellow-500'
                        : 'bg-primary-500'
                  ]"
                  :style="{ width: `${Math.min((row.quota_used / row.quota) * 100, 100)}%` }"
                />
              </div>
            </div>
          </template>

          <template #cell-expires_at="{ value }">
            <span
              v-if="value"
              :class="[
                'text-sm',
                new Date(value) < new Date()
                  ? 'text-red-500 dark:text-red-400'
                  : 'text-gray-500 dark:text-dark-400'
              ]"
            >
              {{ formatDateTime(value) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">
              {{ t('admin.keyIssuance.expiresNever') }}
            </span>
          </template>

          <template #cell-status="{ value }">
            <span :class="['badge', keyStatusClass(value)]">
              {{ keyStatusLabel(value) }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400"
                :title="t('keys.useKey')"
                @click="openUseKeyModal(row)"
              >
                <Icon name="terminal" size="sm" />
                <span class="text-xs">{{ t('keys.useKey') }}</span>
              </button>
              <button
                v-if="!publicSettings?.hide_ccs_import_button"
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                :title="t('keys.importToCcSwitch')"
                @click="importToCcswitch(row)"
              >
                <Icon name="upload" size="sm" />
                <span class="text-xs">{{ t('keys.importToCcSwitch') }}</span>
              </button>
              <button
                type="button"
                :class="[
                  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors disabled:cursor-not-allowed disabled:opacity-40',
                  row.status === 'active'
                    ? 'text-gray-500 hover:bg-yellow-50 hover:text-yellow-600 dark:hover:bg-yellow-900/20 dark:hover:text-yellow-400'
                    : 'text-gray-500 hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400'
                ]"
                :title="row.status === 'active' ? t('keys.disable') : t('keys.enable')"
                @click="toggleKeyStatus(row)"
              >
                <Icon v-if="row.status === 'active'" name="ban" size="sm" />
                <Icon v-else name="checkCircle" size="sm" />
                <span class="text-xs">{{ row.status === 'active' ? t('keys.disable') : t('keys.enable') }}</span>
              </button>
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('common.edit')"
                @click="editKey(row)"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit') }}</span>
              </button>
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
                @click="confirmDelete(row)"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-emerald-50 hover:text-emerald-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-400"
                :disabled="row.status !== 'active' || !row.key"
                :title="row.status === 'active' ? t('admin.keyIssuance.directIssue') : t('admin.keyIssuance.keyUnavailable')"
                @click="deliverExistingKey(row)"
              >
                <Icon name="key" size="sm" />
                <span class="text-xs">{{ t('admin.keyIssuance.directIssue') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <div class="flex flex-col items-center py-12">
              <Icon name="key" size="xl" class="mb-3 text-gray-300 dark:text-dark-500" />
              <p class="font-medium text-gray-900 dark:text-white">
                {{ t('admin.keyIssuance.noExistingKeys') }}
              </p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('admin.keyIssuance.createNewKeyHint') }}
              </p>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <section v-if="result" class="card mt-6 overflow-hidden">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ resultSource === 'existing' ? t('admin.keyIssuance.existingResultTitle') : t('admin.keyIssuance.resultTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.keyIssuance.issuedFor') }}: {{ resultUserLabel }}
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

    <BaseDialog
      v-if="result"
      :show="showDeliveryDialog"
      :title="resultSource === 'existing' ? t('admin.keyIssuance.existingResultTitle') : t('admin.keyIssuance.resultTitle')"
      width="wide"
      @close="closeDeliveryDialog"
    >
      <div class="space-y-5">
        <div class="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 dark:border-emerald-900/60 dark:bg-emerald-900/20">
          <div>
            <p class="font-medium text-emerald-800 dark:text-emerald-200">
              {{ t('admin.keyIssuance.issuedFor') }}: {{ resultUserLabel }}
            </p>
            <p class="mt-1 text-sm text-emerald-700 dark:text-emerald-300">
              {{ resultSource === 'existing' ? t('admin.keyIssuance.existingResultWarning') : t('admin.keyIssuance.resultWarning') }}
            </p>
          </div>
          <Icon name="check" size="md" class="text-emerald-600 dark:text-emerald-400" />
        </div>

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

      <template #footer>
        <div class="flex justify-end">
          <button type="button" class="btn btn-secondary" @click="closeDeliveryDialog">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showCreateDialog"
      :title="t('admin.keyIssuance.createNewKey')"
      width="wide"
      @close="showCreateDialog = false"
    >
      <form id="admin-key-form" class="space-y-5" @submit.prevent="issueKey">
        <div>
          <label class="input-label">{{ t('admin.keyIssuance.recipient') }}</label>
          <Select
            v-model="form.user_id"
            :options="userOptions"
            :loading="loadingUsers"
            :placeholder="t('admin.keyIssuance.searchUsers')"
            remote
            :search-placeholder="t('admin.keyIssuance.searchUsers')"
            @search="handleUserSearch"
          />
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
            :searchable="true"
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
          <button type="button" class="btn btn-secondary" @click="showCreateDialog = false">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="admin-key-form" class="btn btn-primary" :disabled="issuing || loadingUsers || loadingGroups">
            <Icon v-if="issuing" name="refresh" size="md" class="mr-2 animate-spin" />
            <Icon v-else name="key" size="md" class="mr-2" />
            {{ issuing ? t('admin.keyIssuance.issuing') : t('admin.keyIssuance.issue') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showEditDialog"
      :title="t('keys.editKey')"
      width="wide"
      @close="closeEditDialog"
    >
      <form id="admin-edit-key-form" class="space-y-5" @submit.prevent="handleEditSubmit">
        <div>
          <label class="input-label">{{ t('keys.nameLabel') }}</label>
          <input v-model="editForm.name" type="text" required class="input w-full" :placeholder="t('keys.namePlaceholder')" maxlength="100" />
        </div>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('keys.groupLabel') }}</label>
            <Select v-model="editForm.group_id" :options="editGroupOptions" :loading="loadingGroups" :placeholder="t('keys.selectGroup')" :searchable="true" :search-placeholder="t('keys.searchGroup')" />
          </div>
          <div>
            <label class="input-label">{{ t('keys.statusLabel') }}</label>
            <Select v-model="editForm.status" :options="editStatusOptions" :placeholder="t('keys.selectStatus')" />
          </div>
        </div>

        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.ipRestriction') }}</label>
            <button type="button" :aria-pressed="editForm.enable_ip_restriction" class="relative inline-flex h-5 w-9 flex-shrink-0 rounded-full border-2 border-transparent transition-colors focus:outline-none" :class="editForm.enable_ip_restriction ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'" @click="editForm.enable_ip_restriction = !editForm.enable_ip_restriction">
              <span class="pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow ring-0 transition" :class="editForm.enable_ip_restriction ? 'translate-x-4' : 'translate-x-0'" />
            </button>
          </div>
          <div v-if="editForm.enable_ip_restriction" class="space-y-4 pt-2">
            <div>
              <label class="input-label">{{ t('keys.ipWhitelist') }}</label>
              <textarea v-model="editForm.ip_whitelist" rows="3" class="input w-full font-mono text-sm" :placeholder="t('keys.ipWhitelistPlaceholder')" />
              <p class="input-hint">{{ t('keys.ipWhitelistHint') }}</p>
            </div>
            <div>
              <label class="input-label">{{ t('keys.ipBlacklist') }}</label>
              <textarea v-model="editForm.ip_blacklist" rows="3" class="input w-full font-mono text-sm" :placeholder="t('keys.ipBlacklistPlaceholder')" />
              <p class="input-hint">{{ t('keys.ipBlacklistHint') }}</p>
            </div>
          </div>
        </div>

        <div class="space-y-3">
          <label class="input-label">{{ t('keys.quotaLimit') }}</label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
            <input v-model.number="editForm.quota" type="number" min="0" step="0.01" class="input w-full pl-7" :placeholder="t('keys.quotaAmountPlaceholder')" />
          </div>
          <p class="input-hint">{{ t('keys.quotaAmountHint') }}</p>
        </div>

        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.rateLimitSection') }}</label>
            <button type="button" :aria-pressed="editForm.enable_rate_limit" class="relative inline-flex h-5 w-9 flex-shrink-0 rounded-full border-2 border-transparent transition-colors focus:outline-none" :class="editForm.enable_rate_limit ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'" @click="editForm.enable_rate_limit = !editForm.enable_rate_limit">
              <span class="pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow ring-0 transition" :class="editForm.enable_rate_limit ? 'translate-x-4' : 'translate-x-0'" />
            </button>
          </div>
          <p v-if="editForm.enable_rate_limit" class="input-hint">{{ t('keys.rateLimitHint') }}</p>
          <div v-if="editForm.enable_rate_limit" class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div>
              <label class="input-label">{{ t('keys.rateLimit5h') }}</label>
              <input v-model.number="editForm.rate_limit_5h" type="number" min="0" step="0.01" class="input w-full" />
            </div>
            <div>
              <label class="input-label">{{ t('keys.rateLimit1d') }}</label>
              <input v-model.number="editForm.rate_limit_1d" type="number" min="0" step="0.01" class="input w-full" />
            </div>
            <div>
              <label class="input-label">{{ t('keys.rateLimit7d') }}</label>
              <input v-model.number="editForm.rate_limit_7d" type="number" min="0" step="0.01" class="input w-full" />
            </div>
          </div>
        </div>

        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.expiration') }}</label>
            <button type="button" :aria-pressed="editForm.enable_expiration" class="relative inline-flex h-5 w-9 flex-shrink-0 rounded-full border-2 border-transparent transition-colors focus:outline-none" :class="editForm.enable_expiration ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'" @click="editForm.enable_expiration = !editForm.enable_expiration">
              <span class="pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow ring-0 transition" :class="editForm.enable_expiration ? 'translate-x-4' : 'translate-x-0'" />
            </button>
          </div>
          <div v-if="editForm.enable_expiration" class="max-w-sm">
            <label class="input-label">{{ t('keys.expirationDate') }}</label>
            <input v-model="editForm.expiration_date" type="datetime-local" class="input w-full" />
            <p class="input-hint">{{ t('keys.expirationDateHint') }}</p>
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeEditDialog">{{ t('common.cancel') }}</button>
          <button type="submit" form="admin-edit-key-form" class="btn btn-primary" :disabled="submitting || loadingGroups">
            <Icon v-if="submitting" name="refresh" size="md" class="mr-2 animate-spin" />
            <Icon v-else name="check" size="md" class="mr-2" />
            {{ submitting ? t('keys.saving') : t('common.update') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('keys.deleteKey')"
      :message="t('keys.deleteConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />

    <UseKeyModal
      :show="showUseKeyModal"
      :api-key="selectedKey?.key || ''"
      :base-url="publicSettings?.api_base_url || ''"
      :platform="selectedKey?.group?.platform || null"
      :allow-messages-dispatch="selectedKey?.group?.allow_messages_dispatch || false"
      @close="closeUseKeyModal"
    />

    <BaseDialog
      :show="showCcsClientSelect"
      :title="t('keys.ccsClientSelect.title')"
      width="narrow"
      @close="closeCcsClientSelect"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-400">{{ t('keys.ccsClientSelect.description') }}</p>
        <div class="grid grid-cols-2 gap-3">
          <button type="button" class="flex flex-col items-center gap-2 rounded-xl border-2 border-gray-200 p-4 transition-all hover:border-primary-500 hover:bg-primary-50 dark:border-dark-600 dark:hover:border-primary-500 dark:hover:bg-primary-900/20" @click="handleCcsClientSelect('claude')">
            <Icon name="terminal" size="xl" class="text-gray-600 dark:text-gray-400" />
            <span class="font-medium text-gray-900 dark:text-white">{{ t('keys.ccsClientSelect.claudeCode') }}</span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.ccsClientSelect.claudeCodeDesc') }}</span>
          </button>
          <button type="button" class="flex flex-col items-center gap-2 rounded-xl border-2 border-gray-200 p-4 transition-all hover:border-primary-500 hover:bg-primary-50 dark:border-dark-600 dark:hover:border-primary-500 dark:hover:bg-primary-900/20" @click="handleCcsClientSelect('gemini')">
            <Icon name="sparkles" size="xl" class="text-gray-600 dark:text-gray-400" />
            <span class="font-medium text-gray-900 dark:text-white">{{ t('keys.ccsClientSelect.geminiCli') }}</span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.ccsClientSelect.geminiCliDesc') }}</span>
          </button>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end"><button type="button" class="btn btn-secondary" @click="closeCcsClientSelect">{{ t('common.cancel') }}</button></div>
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
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import UseKeyModal from '@/components/keys/UseKeyModal.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { getPublicSettings } from '@/api/auth'
import type { AdminGroup, AdminUser, ApiKey, PublicSettings, UpdateApiKeyRequest } from '@/types'
import type { IssueAPIKeyResponse } from '@/api/admin/apiKeys'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import { buildCcSwitchImportDeeplink, type CcSwitchClientType } from '@/utils/ccswitchImport'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const apiKeys = ref<ApiKey[]>([])
const users = ref<AdminUser[]>([])
const allGroups = ref<AdminGroup[]>([])
const activeGroups = ref<AdminGroup[]>([])
const publicSettings = ref<PublicSettings | null>(null)
const loading = ref(false)
const loadingUsers = ref(false)
const loadingGroups = ref(false)
const issuing = ref(false)
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showDeleteDialog = ref(false)
const showUseKeyModal = ref(false)
const showCcsClientSelect = ref(false)
const showDeliveryDialog = ref(false)
const selectedKey = ref<ApiKey | null>(null)
const pendingCcsRow = ref<ApiKey | null>(null)
const submitting = ref(false)
const copiedKeyId = ref<number | null>(null)
const result = ref<IssueAPIKeyResponse | null>(null)
const resultSource = ref<'existing' | 'new' | null>(null)
const resultUserLabel = ref('')
const issueOperationKey = ref<string | null>(null)
const pinnedUser = ref<AdminUser | null>(null)
let keysRequestId = 0
let usersRequestId = 0
let usersRequestAbort: AbortController | null = null

const filterSearch = ref('')
const filterGroupId = ref<string | number>('')
const filterStatus = ref('')

const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0
})

const form = reactive({
  user_id: null as number | null,
  name: '',
  group_id: null as number | null,
  quota: 0,
  expires_in_days: 0,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0
})

const editForm = reactive({
  name: '',
  group_id: null as number | null,
  status: 'active' as 'active' | 'inactive',
  enable_ip_restriction: false,
  ip_whitelist: '',
  ip_blacklist: '',
  quota: null as number | null,
  enable_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  enable_expiration: false,
  expiration_date: ''
})

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.keyIssuance.user'), sortable: false, class: 'min-w-48' },
  { key: 'name', label: t('common.name'), sortable: false, class: 'min-w-40' },
  { key: 'key', label: t('keys.apiKey'), sortable: false, class: 'min-w-52' },
  { key: 'group', label: t('keys.group'), sortable: false, class: 'min-w-36' },
  { key: 'current_concurrency', label: t('keys.currentConcurrency'), sortable: false },
  { key: 'usage', label: t('keys.usage'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: false },
  { key: 'status', label: t('common.status'), sortable: false },
  { key: 'created_at', label: t('keys.created'), sortable: false },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

const groupFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.keyIssuance.allGroups') },
  { value: 0, label: t('admin.keyIssuance.noGroup') },
  ...allGroups.value.map((group) => ({ value: group.id, label: group.name }))
])

const statusFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.keyIssuance.allStatuses') },
  { value: 'active', label: t('admin.keyIssuance.statusActive') },
  { value: 'inactive', label: t('admin.keyIssuance.statusInactive') },
  { value: 'quota_exhausted', label: t('admin.keyIssuance.statusQuotaExhausted') },
  { value: 'expired', label: t('admin.keyIssuance.statusExpired') }
])

const userOptions = computed<SelectOption[]>(() => {
  const options = users.value.map((user) => ({
    value: user.id,
    label: `${userLabel(user)} · ${user.email}`
  }))
  if (pinnedUser.value && !users.value.some((user) => user.id === pinnedUser.value?.id)) {
    options.unshift({
      value: pinnedUser.value.id,
      label: `${userLabel(pinnedUser.value)} · ${pinnedUser.value.email}`
    })
  }
  return options
})

const groupOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.keyIssuance.noGroup') },
  ...activeGroups.value.map((group) => ({
    value: group.id,
    label: `${group.name} (#${group.id})`
  }))
])

const editGroupOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.keyIssuance.noGroup') },
  ...allGroups.value.map((group) => ({
    value: group.id,
    label: `${group.name} (#${group.id})`
  }))
])

const editStatusOptions = computed<SelectOption[]>(() => [
  { value: 'active', label: t('admin.keyIssuance.statusActive') },
  { value: 'inactive', label: t('admin.keyIssuance.statusInactive') }
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

function userLabel(user: AdminUser): string {
  return user.username?.trim() || user.email
}

function keyUserLabel(key: ApiKey): string {
  return key.user?.username?.trim() || key.user?.email || `#${key.user_id}`
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

async function loadKeys(): Promise<void> {
  const requestId = ++keysRequestId
  loading.value = true
  try {
    const response = await adminAPI.apiKeys.listAdminAPIKeys(pagination.page, pagination.page_size, {
      search: filterSearch.value.trim() || undefined,
      status: filterStatus.value || undefined,
      group_id: filterGroupId.value === '' ? undefined : Number(filterGroupId.value)
    })
    if (requestId !== keysRequestId) return
    apiKeys.value = response.items || []
    pagination.total = response.total || 0
  } catch (error) {
    if (requestId !== keysRequestId) return
    apiKeys.value = []
    pagination.total = 0
    appStore.showError(extractApiErrorMessage(error, t('admin.keyIssuance.failedToLoadExistingKeys')))
  } finally {
    if (requestId === keysRequestId) loading.value = false
  }
}

async function loadUsers(search = ''): Promise<void> {
  const query = search.trim()
  const requestId = ++usersRequestId
  usersRequestAbort?.abort()
  const controller = new AbortController()
  usersRequestAbort = controller
  loadingUsers.value = true
  try {
    const response = await adminAPI.users.list(1, 1000, {
      status: 'active',
      role: 'user',
      search: query || undefined,
      sort_by: 'email',
      sort_order: 'asc'
    }, { signal: controller.signal })
    if (requestId !== usersRequestId) return
    users.value = response.items || []
  } catch (error) {
    if (controller.signal.aborted) return
    if (requestId !== usersRequestId) return
    users.value = []
    appStore.showError(extractApiErrorMessage(error, t('admin.keyIssuance.failedToLoadUsers')))
  } finally {
    if (requestId === usersRequestId) loadingUsers.value = false
  }
}

async function loadGroups(): Promise<void> {
  loadingGroups.value = true
  try {
    const [all, active] = await Promise.all([
      adminAPI.groups.getAllIncludingInactive(),
      adminAPI.groups.getAll()
    ])
    allGroups.value = all
    activeGroups.value = active
  } catch (error) {
    allGroups.value = []
    activeGroups.value = []
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

function onFilterChange(): void {
  pagination.page = 1
  void loadKeys()
}

function onGroupFilterChange(value: string | number | boolean | null): void {
  filterGroupId.value = value === null ? '' : value as string | number
  onFilterChange()
}

function onStatusFilterChange(value: string | number | boolean | null): void {
  filterStatus.value = value === null ? '' : String(value)
  onFilterChange()
}

function handlePageChange(page: number): void {
  pagination.page = page
  void loadKeys()
}

function handlePageSizeChange(pageSize: number): void {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadKeys()
}

async function refreshPage(): Promise<void> {
  await Promise.all([loadKeys(), loadUsers(), loadGroups()])
}

function openCreateDialog(): void {
  resetForm()
  showCreateDialog.value = true
  void loadUsers()
}

function resetForm(): void {
  form.user_id = null
  pinnedUser.value = null
  form.name = ''
  form.group_id = null
  form.quota = 0
  form.expires_in_days = 0
  form.rate_limit_5h = 0
  form.rate_limit_1d = 0
  form.rate_limit_7d = 0
  issueOperationKey.value = null
}

function handleUserSearch(query: string): void {
  void loadUsers(query)
}

watch(() => form.user_id, (userId) => {
  if (userId === null) {
    pinnedUser.value = null
    return
  }
  const selectedUser = users.value.find((user) => user.id === userId)
  if (selectedUser) pinnedUser.value = selectedUser
})

async function issueKey(): Promise<void> {
  if (!form.user_id) {
    appStore.showError(t('admin.keyIssuance.userRequired'))
    return
  }

  const name = form.name.trim()
  if (!name) {
    appStore.showError(t('admin.keyIssuance.nameRequired'))
    return
  }

  const selectedUser = users.value.find((user) => user.id === form.user_id) ||
    (pinnedUser.value?.id === form.user_id ? pinnedUser.value : undefined)
  if (!selectedUser || selectedUser.status !== 'active') {
    appStore.showError(t('admin.keyIssuance.userInactive'))
    return
  }

  issuing.value = true
  try {
    result.value = await adminAPI.apiKeys.issueAPIKey(
      {
        user_id: selectedUser.id,
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
    resultUserLabel.value = userLabel(selectedUser)
    showCreateDialog.value = false
    showDeliveryDialog.value = true
    appStore.showSuccess(t('admin.keyIssuance.keyCreated'))
    await loadKeys()
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
  resultUserLabel.value = keyUserLabel(key)
  issueOperationKey.value = null
  showDeliveryDialog.value = true
  appStore.showSuccess(t('admin.keyIssuance.existingKeySelected'))
}

function closeDeliveryDialog(): void {
  showDeliveryDialog.value = false
}

function openUseKeyModal(key: ApiKey): void {
  selectedKey.value = key
  showUseKeyModal.value = true
}

function closeUseKeyModal(): void {
  showUseKeyModal.value = false
  selectedKey.value = null
}

function formatDateTimeLocal(isoDate: string): string {
  const date = new Date(isoDate)
  const pad = (value: number) => value.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function editKey(key: ApiKey): void {
  selectedKey.value = key
  editForm.name = key.name
  editForm.group_id = key.group_id
  editForm.status = key.status === 'quota_exhausted' || key.status === 'expired' ? 'inactive' : key.status
  editForm.enable_ip_restriction = (key.ip_whitelist?.length || 0) > 0 || (key.ip_blacklist?.length || 0) > 0
  editForm.ip_whitelist = (key.ip_whitelist || []).join('\n')
  editForm.ip_blacklist = (key.ip_blacklist || []).join('\n')
  editForm.quota = key.quota > 0 ? key.quota : null
  editForm.enable_rate_limit = key.rate_limit_5h > 0 || key.rate_limit_1d > 0 || key.rate_limit_7d > 0
  editForm.rate_limit_5h = key.rate_limit_5h > 0 ? key.rate_limit_5h : null
  editForm.rate_limit_1d = key.rate_limit_1d > 0 ? key.rate_limit_1d : null
  editForm.rate_limit_7d = key.rate_limit_7d > 0 ? key.rate_limit_7d : null
  editForm.enable_expiration = !!key.expires_at
  editForm.expiration_date = key.expires_at ? formatDateTimeLocal(key.expires_at) : ''
  showEditDialog.value = true
}

function closeEditDialog(): void {
  showEditDialog.value = false
  selectedKey.value = null
}

function shouldSubmitEditStatus(key: ApiKey, status: 'active' | 'inactive'): boolean {
  if (key.status === 'quota_exhausted' || key.status === 'expired') return status === 'active'
  return true
}

async function toggleKeyStatus(key: ApiKey): Promise<void> {
  const newStatus = key.status === 'active' ? 'inactive' : 'active'
  try {
    await adminAPI.apiKeys.updateAPIKey(key.id, { status: newStatus })
    appStore.showSuccess(newStatus === 'active' ? t('keys.keyEnabledSuccess') : t('keys.keyDisabledSuccess'))
    await loadKeys()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('keys.failedToUpdateStatus')))
  }
}

function confirmDelete(key: ApiKey): void {
  selectedKey.value = key
  showDeleteDialog.value = true
}

async function handleDelete(): Promise<void> {
  if (!selectedKey.value) return
  const keyId = selectedKey.value.id
  try {
    await adminAPI.apiKeys.deleteAPIKey(keyId)
    showDeleteDialog.value = false
    selectedKey.value = null
    appStore.showSuccess(t('keys.keyDeletedSuccess'))
    await loadKeys()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('keys.failedToDelete')))
  }
}

async function handleEditSubmit(): Promise<void> {
  if (!selectedKey.value) return
  const name = editForm.name.trim()
  if (!name) {
    appStore.showError(t('admin.keyIssuance.nameRequired'))
    return
  }

  const parseIPList = (value: string): string[] => value.split('\n').map((item) => item.trim()).filter(Boolean)
  const updates: UpdateApiKeyRequest = {
    name,
    ip_whitelist: editForm.enable_ip_restriction ? parseIPList(editForm.ip_whitelist) : [],
    ip_blacklist: editForm.enable_ip_restriction ? parseIPList(editForm.ip_blacklist) : [],
    quota: editForm.quota && editForm.quota > 0 ? editForm.quota : 0,
    rate_limit_5h: editForm.enable_rate_limit && editForm.rate_limit_5h && editForm.rate_limit_5h > 0 ? editForm.rate_limit_5h : 0,
    rate_limit_1d: editForm.enable_rate_limit && editForm.rate_limit_1d && editForm.rate_limit_1d > 0 ? editForm.rate_limit_1d : 0,
    rate_limit_7d: editForm.enable_rate_limit && editForm.rate_limit_7d && editForm.rate_limit_7d > 0 ? editForm.rate_limit_7d : 0
  }

  if (editForm.group_id !== selectedKey.value.group_id) {
    updates.group_id = editForm.group_id === null ? 0 : editForm.group_id
  }
  if (shouldSubmitEditStatus(selectedKey.value, editForm.status)) {
    updates.status = editForm.status
  }
  if (editForm.enable_expiration && editForm.expiration_date) {
    updates.expires_at = new Date(editForm.expiration_date).toISOString()
  } else {
    updates.clear_expiration = true
  }

  submitting.value = true
  try {
    await adminAPI.apiKeys.updateAPIKey(selectedKey.value.id, updates)
    appStore.showSuccess(t('keys.keyUpdatedSuccess'))
    closeEditDialog()
    await loadKeys()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('keys.failedToSave')))
  } finally {
    submitting.value = false
  }
}

function importToCcswitch(row: ApiKey): void {
  const platform = row.group?.platform || 'anthropic'
  if (platform === 'antigravity') {
    pendingCcsRow.value = row
    showCcsClientSelect.value = true
    return
  }
  executeCcsImport(row, platform === 'gemini' ? 'gemini' : 'claude')
}

function executeCcsImport(row: ApiKey, clientType: CcSwitchClientType): void {
  const baseUrl = publicSettings.value?.api_base_url || window.location.origin
  const platform = row.group?.platform || 'anthropic'
  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return { isValid: response?.is_active ?? response?.isValid ?? true, remaining, unit };
    }
  })`
  const providerName = (publicSettings.value?.site_name || 'sub2api').trim() || 'sub2api'
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl,
    platform,
    clientType,
    providerName,
    apiKey: row.key,
    usageScript
  })
  try {
    window.open(deeplink, '_self')
    window.setTimeout(() => {
      if (document.hasFocus()) appStore.showError(t('keys.ccSwitchNotInstalled'))
    }, 100)
  } catch {
    appStore.showError(t('keys.ccSwitchNotInstalled'))
  }
}

function handleCcsClientSelect(clientType: CcSwitchClientType): void {
  if (pendingCcsRow.value) executeCcsImport(pendingCcsRow.value, clientType)
  closeCcsClientSelect()
}

function closeCcsClientSelect(): void {
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
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

onMounted(() => {
  void Promise.all([loadKeys(), loadUsers(), loadGroups(), loadSettings()])
})
</script>
