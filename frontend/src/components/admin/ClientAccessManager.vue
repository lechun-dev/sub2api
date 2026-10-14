<template>
  <section class="space-y-6" :aria-busy="loading || saving">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold">{{ t('admin.clientAccess.title') }}</h1>
        <p class="mt-2 text-sm text-gray-500">{{ t('admin.clientAccess.description') }}</p>
      </div>
      <button type="button" class="btn btn-secondary" :disabled="loading || saving" @click="reload">{{ t('admin.clientAccess.reload') }}</button>
    </header>
    <p class="rounded-lg bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ t('admin.clientAccess.risk') }}</p>
    <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
    <p v-if="loading">{{ t('common.loading') }}</p>
    <form v-else-if="config" class="space-y-6" @submit.prevent="save">
      <fieldset :disabled="saving" class="space-y-6">
        <section class="card p-5 space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 class="text-lg font-semibold">{{ t('admin.clientAccess.clients') }}</h2>
            <button type="button" class="btn btn-secondary" :disabled="config.clients.length >= 100" @click="addClient">{{ t('admin.clientAccess.addClient') }}</button>
          </div>
          <details v-for="(client, index) in config.clients" :key="index" class="rounded-lg border border-gray-200 p-4 dark:border-dark-600" :open="!knownIds.has(client.id)">
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
        <section class="card p-5 space-y-4">
          <div class="flex flex-wrap justify-between gap-3"><h2 class="text-lg font-semibold">{{ t('admin.clientAccess.groups') }}</h2><span class="text-sm text-gray-500">{{ t('admin.clientAccess.policyCount', { count: restrictedCount }) }}</span></div>
          <p class="text-sm text-gray-500">{{ t('admin.clientAccess.unknownHint') }}</p>
          <label class="block">{{ t('admin.clientAccess.searchGroup') }}<input v-model="search" class="input mt-1" type="search" /></label>
          <label class="block">{{ t('admin.clientAccess.selectGroup') }}
            <select v-model="selectedGroup" class="input mt-1"><option value="">{{ t('admin.clientAccess.selectGroup') }}</option><option v-for="group in filteredGroups" :key="group.id" :value="String(group.id)">{{ group.name }} · ID {{ group.id }} · {{ modeLabel(config.groups[String(group.id)]?.mode ?? 'off') }}</option></select>
          </label>
          <p v-if="groups.length === 0" class="text-sm text-gray-500">{{ t('admin.clientAccess.noGroups') }}</p>
          <div v-if="selectedGroup && selectedPolicy" class="space-y-4">
            <p class="text-xs text-gray-500">{{ t('admin.clientAccess.groupId') }}: {{ selectedGroup }}</p>
            <select :value="selectedPolicy.mode" class="input" :aria-label="t('admin.clientAccess.groups')" @change="setMode(($event.target as HTMLSelectElement).value as GroupClientPolicy['mode'])"><option value="off">{{ modeLabel('off') }}</option><option value="allow">{{ modeLabel('allow') }}</option><option value="deny">{{ modeLabel('deny') }}</option></select>
            <div v-if="selectedPolicy.mode !== 'off'" class="grid gap-3 sm:grid-cols-2">
              <label v-for="client in config.clients" :key="client.id" class="flex items-center gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600"><input type="checkbox" :checked="selectedPolicy.client_ids.includes(client.id)" :disabled="!client.enabled" @change="toggleClient(client.id, ($event.target as HTMLInputElement).checked)" />{{ client.name }}</label>
            </div>
            <div v-if="selectedPolicy.mode !== 'off'" class="rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800"><strong>{{ t('admin.clientAccess.preview') }}</strong><p class="mt-2">{{ preview }}</p></div>
          </div>
        </section>
        <footer class="flex flex-wrap items-center justify-between gap-3">
          <span class="text-sm text-amber-600">{{ dirty ? t('admin.clientAccess.unsaved') : '' }}</span>
          <button type="submit" class="btn btn-primary" :disabled="!dirty || saving" data-testid="save-client-access">{{ saving ? t('common.loading') : t('admin.clientAccess.save') }}</button>
        </footer>
      </fieldset>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getAllIncludingInactive } from '@/api/admin/groups'
import { getClientAccess, saveClientAccess, referencingGroups, policyPreview } from '@/api/admin/clientAccess'
import type { ClientAccessConfig, GroupClientPolicy } from '@/api/admin/clientAccess'
import type { AdminGroup } from '@/types'

const props = defineProps<{ groupId?: string }>()
const { t } = useI18n()
const app = useAppStore()
const config = ref<ClientAccessConfig | null>(null)
const groups = ref<AdminGroup[]>([])
const selectedGroup = ref(props.groupId ?? '')
const search = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const baseline = ref('')
const knownIds = ref(new Set<string>())
const dirty = computed(() => config.value !== null && JSON.stringify(config.value) !== baseline.value)
const filteredGroups = computed(() => groups.value.filter(group => String(group.id) === selectedGroup.value || `${group.id} ${group.name}`.toLowerCase().includes(search.value.toLowerCase())))
const selectedPolicy = computed(() => selectedGroup.value ? config.value?.groups[selectedGroup.value] ?? { mode: 'off', client_ids: [] } as GroupClientPolicy : null)
const restrictedCount = computed(() => Object.values(config.value?.groups ?? {}).filter(policy => policy.mode !== 'off').length)
const preview = computed(() => selectedPolicy.value && config.value ? t(`admin.clientAccess.${selectedPolicy.value.mode}Preview`, { clients: policyPreview(selectedPolicy.value, config.value.clients) }) : '')
const modeLabel = (mode: GroupClientPolicy['mode']) => t(`admin.clientAccess.${mode}`)
const references = (id: string) => config.value ? referencingGroups(config.value, id) : []
function referenceText(id: string) {
  const ids = references(id)
  return ids.length ? t('admin.clientAccess.referenced', { groups: ids.map(key => `${groups.value.find(group => String(group.id) === key)?.name ?? key} (ID ${key})`).join('、') }) : t('admin.clientAccess.noReferences')
}
function setMode(mode: GroupClientPolicy['mode']) {
  if (!config.value || !selectedGroup.value) return
  if (mode === 'off') { delete config.value.groups[selectedGroup.value]; return }
  config.value.groups[selectedGroup.value] = { mode, client_ids: selectedPolicy.value?.client_ids.slice() ?? [] }
}
function toggleClient(id: string, checked: boolean) {
  const policy = config.value?.groups[selectedGroup.value]
  if (!policy) return
  policy.client_ids = checked ? [...new Set([...policy.client_ids, id])] : policy.client_ids.filter(value => value !== id)
}
function addClient() {
  config.value?.clients.push({ id: '', name: '', enabled: true, detector: 'headers', match: 'all', rules: [{ header: 'User-Agent', operator: 'prefix', value: '' }] })
}
function removeClient(index: number) {
  if (!config.value || references(config.value.clients[index].id).length) return
  config.value.clients.splice(index, 1)
}
async function reload() {
  if (dirty.value && !window.confirm(t('admin.clientAccess.discard'))) return
  loading.value = true
  error.value = ''
  try {
    const [next, liveGroups] = await Promise.all([getClientAccess(), getAllIncludingInactive()])
    config.value = next
    groups.value = liveGroups
    knownIds.value = new Set(next.clients.map(client => client.id))
    baseline.value = JSON.stringify(next)
    if (selectedGroup.value && !liveGroups.some(group => String(group.id) === selectedGroup.value)) selectedGroup.value = ''
  } catch { error.value = t('admin.clientAccess.loadError'); config.value = null }
  finally { loading.value = false }
}
async function save() {
  if (!config.value || saving.value) return
  error.value = ''
  if (Object.values(config.value.groups).some(policy => policy.mode !== 'off' && (policy.client_ids.length === 0 || policy.client_ids.some(id => !config.value?.clients.some(client => client.id === id && client.enabled))))) { error.value = t('admin.clientAccess.required'); return }
  if (config.value.clients.some(client => !/^[a-z][a-z0-9_-]{0,63}$/.test(client.id) || !client.name.trim() || (client.detector === 'headers' && (!client.rules.length || client.rules.some(rule => !rule.header.trim() || !rule.value.trim()))))) { error.value = t('admin.clientAccess.invalidClient'); return }
  saving.value = true
  try {
    await saveClientAccess(config.value)
    // 2026-10-13 coder(lq): 保存后必须回读新 revision，否则下一次编辑会使用过期并发版本。
    baseline.value = JSON.stringify(config.value)
    await reload()
    if (!error.value) app.showSuccess(t('admin.clientAccess.saved'))
  } catch (err: unknown) {
    const failure = err as { status?: number; message?: string }
    error.value = failure.status === 409 ? t('admin.clientAccess.conflict') : failure.message || t('admin.clientAccess.saveError')
  } finally { saving.value = false }
}
onMounted(reload)
defineExpose({ dirty })
</script>
