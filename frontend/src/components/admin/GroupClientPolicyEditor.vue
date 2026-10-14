<template>
  <form class="space-y-4" @submit.prevent="$emit('save')">
    <fieldset :disabled="saving" class="space-y-4">
      <slot />
      <p class="text-sm text-gray-500">{{ t('admin.clientAccess.unknownHint') }}</p>
      <label class="block">{{ t('admin.clientAccess.mode') }}
        <select :value="modelValue.mode" class="input mt-1" data-testid="policy-mode" @change="setMode(($event.target as HTMLSelectElement).value as GroupClientPolicy['mode'])">
          <option v-for="mode in (restrictedOnly ? ['allow', 'deny'] : ['off', 'allow', 'deny'])" :key="mode" :value="mode">{{ t(`admin.clientAccess.${mode}`) }}</option>
        </select>
      </label>
      <fieldset v-if="modelValue.mode !== 'off'" class="space-y-3">
        <legend class="mb-2 font-medium">{{ t('admin.clientAccess.selectedClients') }}</legend>
        <div class="grid gap-3 sm:grid-cols-2">
          <label v-for="client in clients" :key="client.id" class="flex items-center gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <input type="checkbox" :checked="modelValue.client_ids.includes(client.id)" :disabled="!client.enabled" :data-client-id="client.id" @change="toggleClient(client.id, ($event.target as HTMLInputElement).checked)" />
            {{ client.name }}<span v-if="!client.enabled" class="text-xs text-gray-500">{{ t('admin.clientAccess.disabledClient') }}</span>
          </label>
        </div>
        <p v-if="!clients.some(client => client.enabled)" class="text-sm text-amber-600">{{ t('admin.clientAccess.noEnabledClients') }}</p>
        <div class="rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800"><strong>{{ t('admin.clientAccess.preview') }}</strong><p class="mt-2">{{ preview }}</p></div>
      </fieldset>
      <footer class="flex flex-wrap items-center justify-between gap-3">
        <span class="text-sm text-amber-600">{{ dirty ? t('admin.clientAccess.unsaved') : '' }}</span>
        <div class="flex gap-2">
          <button v-if="cancelable" type="button" class="btn btn-secondary" @click="$emit('cancel')">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="!dirty || !canSave" data-testid="save-client-access">{{ t(scoped ? 'admin.clientAccess.saveGroup' : 'admin.clientAccess.savePolicy') }}</button>
        </div>
      </footer>
    </fieldset>
  </form>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { policyPreview } from '@/api/admin/clientAccess'
import type { ClientDefinition, GroupClientPolicy } from '@/api/admin/clientAccess'
const props = defineProps<{ modelValue: GroupClientPolicy; clients: ClientDefinition[]; saving: boolean; dirty: boolean; canSave: boolean; scoped?: boolean; cancelable?: boolean; restrictedOnly?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [policy: GroupClientPolicy]; save: []; cancel: [] }>()
const { t } = useI18n()
const preview = computed(() => t(`admin.clientAccess.${props.modelValue.mode}Preview`, { clients: policyPreview(props.modelValue, props.clients) || t('admin.clientAccess.noneSelected') }))
function setMode(mode: GroupClientPolicy['mode']) {
  emit('update:modelValue', { ...props.modelValue, mode, client_ids: mode === 'off' ? [] : props.modelValue.client_ids.slice() })
}
function toggleClient(id: string, checked: boolean) {
  emit('update:modelValue', { ...props.modelValue, client_ids: checked ? [...new Set([...props.modelValue.client_ids, id])] : props.modelValue.client_ids.filter(value => value !== id) })
}
</script>
