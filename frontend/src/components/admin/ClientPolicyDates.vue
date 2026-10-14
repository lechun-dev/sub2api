<template>
  <dl class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
    <div><dt class="inline">{{ t('admin.clientAccess.createdAt') }}: </dt><dd class="inline">{{ date(policy?.created_at) }}</dd></div>
    <div><dt class="inline">{{ t('admin.clientAccess.updatedAt') }}: </dt><dd class="inline">{{ date(policy?.updated_at) }}</dd></div>
  </dl>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { GroupClientPolicy } from '@/api/admin/clientAccess'
import { formatDateTimeToMinute } from '@/utils/format'
defineProps<{ policy?: GroupClientPolicy }>()
const { t } = useI18n()
const date = (value?: string | null) => value ? formatDateTimeToMinute(value) : t('admin.clientAccess.historyUnknown')
</script>
