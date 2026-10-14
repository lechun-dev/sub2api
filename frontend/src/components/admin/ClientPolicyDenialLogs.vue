<template>
  <BaseDialog :show="true" :title="t('admin.clientAccess.logsTitle', { group: groupName, id: groupId })" width="wide" @close="$emit('close')">
    <div class="space-y-4">
      <p class="text-sm text-gray-500">{{ t('admin.clientAccess.logsHint') }}</p>
      <div class="flex flex-wrap items-end gap-3">
        <label>{{ t('admin.clientAccess.logRange') }}
          <select v-model="days" class="input mt-1" data-testid="log-range" @change="refresh">
            <option :value="1">{{ t('admin.clientAccess.last24h') }}</option><option :value="7">{{ t('admin.clientAccess.last7d') }}</option><option :value="30">{{ t('admin.clientAccess.last30d') }}</option>
          </select>
        </label>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="refresh">{{ t('admin.clientAccess.refreshLogs') }}</button>
      </div>
      <p class="text-xs text-gray-500 break-words">{{ formatDateTimeToMinute(start) }} — {{ formatDateTimeToMinute(end) }} · {{ timezone }}</p>
      <p v-if="loading" role="status">{{ t('common.loading') }}</p>
      <p v-else-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <template v-else>
        <p class="text-sm text-gray-500">{{ t('admin.clientAccess.logTotal', { count: total }) }}</p>
        <p v-if="!items.length" class="rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800">{{ t('admin.clientAccess.noLogs') }}</p>
        <ul v-else class="space-y-3">
          <li v-for="item in items" :key="item.id" class="rounded-lg border border-gray-200 p-4 space-y-2 text-sm dark:border-dark-600">
            <div class="flex flex-wrap justify-between gap-2"><strong>{{ formatDateTimeToMinute(item.created_at) }}</strong><span class="text-gray-500">#{{ item.id }} · HTTP 403</span></div>
            <template v-if="item.snapshot">
              <p class="font-medium">{{ t(`admin.clientAccess.reason_${item.snapshot.code}`) }}</p>
              <p v-if="item.snapshot.recognition_reason" class="text-sm text-gray-500">{{ recognitionReason(item.snapshot.recognition_reason) }}</p>
              <p>{{ t('admin.clientAccess.detectedClient') }}: {{ clients(item.snapshot.detected_clients, item.snapshot.detected_count) }}</p>
              <p>{{ t('admin.clientAccess.policyAtDenial') }}: {{ t(`admin.clientAccess.${item.snapshot.mode}`) }} · {{ clients(item.snapshot.selected_clients, item.snapshot.selected_count) }}</p>
              <p class="text-xs text-gray-500">{{ t('admin.clientAccess.updatedAt') }}: {{ item.snapshot.policy_updated_at ? formatDateTimeToMinute(item.snapshot.policy_updated_at) : t('admin.clientAccess.historyUnknown') }}</p>
            </template>
            <p v-else>{{ t('admin.clientAccess.snapshotUnavailable') }}</p>
            <p class="text-xs text-gray-500 break-all"><strong>User-Agent:</strong> {{ userAgentLabel(item) }}</p>
            <dl class="grid gap-2 sm:grid-cols-2 text-xs text-gray-500 break-all">
              <div><dt class="inline">{{ t('admin.clientAccess.logUser') }}: </dt><dd class="inline">{{ item.username?.trim() || t('admin.clientAccess.userUnavailable') }}</dd></div>
              <div><dt class="inline">API Key ID: </dt><dd class="inline">{{ item.api_key_id ?? '—' }}</dd></div>
              <div><dt class="inline">IP: </dt><dd class="inline">{{ item.client_ip || '—' }}</dd></div>
              <div><dt class="inline">{{ t('admin.clientAccess.logRoute') }}: </dt><dd class="inline">{{ item.request_path || '—' }}</dd></div>
              <div class="sm:col-span-2"><dt class="inline">Request ID: </dt><dd class="inline">{{ item.request_id || '—' }}</dd></div>
            </dl>
          </li>
        </ul>
        <div v-if="total > pageSize" class="flex flex-wrap items-center justify-between gap-2">
          <button type="button" class="btn btn-secondary" :disabled="page <= 1" @click="move(-1)">{{ t('admin.clientAccess.previousPage') }}</button>
          <span class="text-sm">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
          <button type="button" class="btn btn-secondary" :disabled="page * pageSize >= total" @click="move(1)">{{ t('admin.clientAccess.nextPage') }}</button>
        </div>
      </template>
    </div>
  </BaseDialog>
</template>
<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getClientPolicyDenials } from '@/api/admin/clientAccess'
import type { ClientPolicyDenialLog } from '@/api/admin/clientAccess'
import { formatDateTimeToMinute } from '@/utils/format'
const props = defineProps<{ groupId: string; groupName: string }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const days = ref(1), page = ref(1), total = ref(0)
const pageSize = 20
const items = ref<ClientPolicyDenialLog[]>([])
const loading = ref(false), error = ref('')
const start = ref(''), end = ref('')
const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
let controller: AbortController | undefined
let sequence = 0
function clients(values: { name: string }[], count: number): string {
  const names = values.map(value => value.name).join('、') || t('admin.clientAccess.unrecognizedClient')
  return count > values.length ? t('admin.clientAccess.moreClients', { names, count }) : names
}
function userAgentLabel(item: ClientPolicyDenialLog): string {
  if (item.user_agent) return item.user_agent
  if (item.snapshot?.user_agent_present === true) return t('admin.clientAccess.userAgentFiltered')
  if (item.snapshot?.user_agent_present === false || item.snapshot?.recognition_reason === 'user_agent_missing') return t('admin.clientAccess.userAgentMissing')
  return t('admin.clientAccess.historyUnknown')
}
function recognitionReason(reason: string): string {
  return ['user_agent_missing', 'no_matching_rule', 'matched_registered_client'].includes(reason) ? t(`admin.clientAccess.recognition_${reason}`) : t('admin.clientAccess.historyUnknown')
}
async function load() {
  // 2026-10-14 coder(lq): 日期分页固定查询快照，切换范围/分组取消旧请求并阻止迟到响应覆盖。
  controller?.abort()
  controller = new AbortController()
  const current = ++sequence
  loading.value = true
  error.value = ''
  items.value = []
  try {
    const result = await getClientPolicyDenials({ group_id: props.groupId, start_time: start.value, end_time: end.value, page: page.value, page_size: pageSize }, controller.signal)
    if (current !== sequence) return
    items.value = result.items
    total.value = result.total
  } catch (failure: unknown) {
    if (current !== sequence) return
    const err = failure as { status?: number; code?: string }
    error.value = t(err.code === 'OPS_DISABLED' ? 'admin.clientAccess.logsDisabled' : 'admin.clientAccess.logsError')
  } finally { if (current === sequence) loading.value = false }
}
function refresh() {
  end.value = new Date().toISOString()
  start.value = new Date(Date.parse(end.value) - days.value * 86400000).toISOString()
  page.value = 1
  void load()
}
function move(delta: number) { page.value += delta; void load() }
watch(() => props.groupId, refresh, { immediate: true })
onUnmounted(() => { ++sequence; controller?.abort() })
</script>
