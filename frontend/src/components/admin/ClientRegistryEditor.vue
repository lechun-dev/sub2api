<template>
  <form class="space-y-4" @submit.prevent="$emit('save', clients)">
    <fieldset :disabled="saving" class="space-y-4">
        <section class="card p-5 space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 class="text-lg font-semibold">{{ t('admin.clientAccess.clients') }}</h2>
            <button type="button" class="btn btn-secondary" :disabled="clients.length >= 100" @click="addClient">{{ t('admin.clientAccess.addClient') }}</button>
          </div>
          <details v-for="(client, index) in clients" :key="index" class="rounded-lg border border-gray-200 p-4 dark:border-dark-600" :open="!knownIds.has(client.id)">
            <summary class="cursor-pointer font-medium">{{ client.name || t('admin.clientAccess.addClient') }} <span class="ml-2 text-xs text-gray-500">{{ client.id }}</span></summary>
            <div class="mt-4 space-y-4">
              <div class="grid gap-3 sm:grid-cols-2">
                <label>{{ t('admin.clientAccess.id') }}<input v-model="client.id" class="input mt-1" required pattern="[a-z][a-z0-9_\-]{0,63}" maxlength="64" :readonly="knownIds.has(client.id)" /></label>
                <label>{{ t('admin.clientAccess.name') }}<input v-model="client.name" class="input mt-1" required maxlength="128" /></label>
              </div>
              <div class="flex items-center justify-between gap-3">
                <label class="flex items-center gap-2"><input v-model="client.enabled" type="checkbox" :disabled="references(client.id).length > 0" />{{ t('admin.clientAccess.enabled') }}</label>
                <button type="button" class="btn btn-secondary" :disabled="references(client.id).length > 0" @click="removeClient(index)">{{ t('admin.clientAccess.remove') }}</button>
              </div>
              <p class="text-sm text-gray-500">{{ referenceText(client.id) }}</p>
              <p v-if="client.detector === 'codex'" class="text-sm text-gray-500">{{ t('admin.clientAccess.builtinHint') }}</p>
              <template v-else>
                <label class="block">{{ t('admin.clientAccess.operator') }}
                  <select v-model="client.match" class="input mt-1"><option value="all">{{ t('admin.clientAccess.all') }}</option><option value="any">{{ t('admin.clientAccess.any') }}</option></select>
                </label>
                <div v-for="(rule, ruleIndex) in client.rules" :key="ruleIndex" class="grid items-end gap-2 sm:grid-cols-[1fr_1fr_2fr_auto]">
                  <label>{{ t('admin.clientAccess.header') }}<input v-model="rule.header" class="input mt-1" required maxlength="64" placeholder="User-Agent" /></label>
                  <label>{{ t('admin.clientAccess.operator') }}<select v-model="rule.operator" class="input mt-1"><option v-for="op in ['exact', 'prefix', 'contains']" :key="op" :value="op">{{ t(`admin.clientAccess.${op}`) }}</option></select></label>
                  <label>{{ t('admin.clientAccess.value') }}<input v-model="rule.value" class="input mt-1" required maxlength="256" /></label>
                  <button type="button" class="btn btn-secondary" :disabled="client.rules.length <= 1" @click="client.rules.splice(ruleIndex, 1)">{{ t('admin.clientAccess.remove') }}</button>
                </div>
                <button type="button" class="btn btn-secondary" :disabled="client.rules.length >= 12" @click="client.rules.push({ header: 'User-Agent', operator: 'prefix', value: '' })">{{ t('admin.clientAccess.addRule') }}</button>
                <p class="text-xs text-gray-500">{{ t('admin.clientAccess.affected') }}</p>
              </template>
            </div>
          </details>
        </section>
      <footer class="flex flex-wrap items-center justify-between gap-3">
        <span class="text-sm text-amber-600">{{ dirty ? t('admin.clientAccess.unsaved') : '' }}</span>
        <button type="submit" class="btn btn-primary" :disabled="!dirty" data-testid="save-clients">{{ t('admin.clientAccess.saveClients') }}</button>
      </footer>
    </fieldset>
  </form>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { referencingGroups } from '@/api/admin/clientAccess'
import type { ClientAccessConfig, ClientDefinition } from '@/api/admin/clientAccess'
import type { AdminGroup } from '@/types'
const props = defineProps<{ config: ClientAccessConfig; groups: AdminGroup[]; saving: boolean }>()
defineEmits<{ save: [clients: ClientDefinition[]] }>()
const { t } = useI18n()
const clients = ref<ClientDefinition[]>([])
const knownIds = computed(() => new Set(props.config.clients.map(client => client.id)))
const dirty = computed(() => JSON.stringify(clients.value) !== JSON.stringify(props.config.clients))
// 2026-10-14 coder(lq): 客户端草稿独立于分组策略，保存客户端时不夹带未保存的分组变更。
watch(() => props.config, value => { clients.value = JSON.parse(JSON.stringify(value.clients)) }, { immediate: true })
const references = (id: string) => referencingGroups(props.config, id)
function referenceText(id: string) {
  const ids = references(id)
  return ids.length ? t('admin.clientAccess.referenced', { groups: ids.map(key => `${props.groups.find(group => String(group.id) === key)?.name ?? key} (ID ${key})`).join('、') }) : t('admin.clientAccess.noReferences')
}
function addClient() {
  clients.value.push({ id: '', name: '', enabled: true, detector: 'headers', match: 'all', rules: [{ header: 'User-Agent', operator: 'prefix', value: '' }] })
}
function removeClient(index: number) {
  if (!references(clients.value[index].id).length) clients.value.splice(index, 1)
}
defineExpose({ dirty })
</script>
