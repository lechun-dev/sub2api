<template>
  <section class="space-y-6" :aria-busy="loading || saving">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold">{{ scoped ? t('admin.clientAccess.groupTitle', { group: currentGroup?.name ?? props.groupId }) : t('admin.clientAccess.title') }}</h1>
        <p class="mt-2 text-sm text-gray-500">{{ t(scoped ? 'admin.clientAccess.groupDescription' : 'admin.clientAccess.description') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <RouterLink v-if="scoped" to="/admin/groups" class="btn btn-secondary">{{ t('admin.clientAccess.backGroups') }}</RouterLink>
        <button type="button" class="btn btn-secondary" :disabled="loading || saving" @click="reload">{{ t('admin.clientAccess.reload') }}</button>
      </div>
    </header>
    <p class="rounded-lg bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ t(scoped ? 'admin.clientAccess.groupRisk' : activeTab === 'clients' ? 'admin.clientAccess.risk' : 'admin.clientAccess.policyRisk') }}</p>
    <p v-if="error || groupError" role="alert" class="rounded-lg bg-red-50 p-3 text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error || groupError }}</p>
    <p v-if="loading">{{ t('common.loading') }}</p>
    <template v-else-if="config">
      <section v-if="scoped && currentGroup" class="card p-5 space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div><h2 class="text-lg font-semibold">{{ currentGroup.name }}</h2><p class="mt-1 text-sm text-gray-500">{{ t('admin.clientAccess.groupId') }}: {{ currentGroup.id }}</p></div>
          <RouterLink :to="{ path: '/admin/client-access', query: { tab: 'clients' } }" class="btn btn-secondary">{{ t('admin.clientAccess.manageClients') }}</RouterLink>
        </div>
        <ClientPolicyDates v-if="config.groups[selectedGroup]" :policy="config.groups[selectedGroup]" />
        <button type="button" class="btn btn-secondary" data-testid="group-policy-logs" @click="logGroup = { id: selectedGroup, name: currentGroup.name }">{{ t('admin.clientAccess.logs') }}</button>
        <GroupClientPolicyEditor v-model="draft" :clients="config.clients" :saving="saving || needsReload" :dirty="policyDirty" :can-save="true" scoped @save="savePolicy" />
      </section>
      <template v-else-if="!scoped">
        <nav class="flex flex-wrap gap-2" :aria-label="t('admin.clientAccess.title')">
          <button v-for="tab in ['policies', 'clients'] as const" :key="tab" type="button" :class="['btn', activeTab === tab ? 'btn-primary' : 'btn-secondary']" :aria-pressed="activeTab === tab" :disabled="saving" @click="switchTab(tab)">{{ t(tab === 'policies' ? 'admin.clientAccess.groups' : 'admin.clientAccess.clients') }}</button>
        </nav>
        <section v-if="activeTab === 'policies'" class="card p-5 space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div><h2 class="text-lg font-semibold">{{ t('admin.clientAccess.groups') }}</h2><p class="mt-1 text-sm text-gray-500">{{ t('admin.clientAccess.policyCount', { count: policies.length }) }}</p></div>
            <button type="button" class="btn btn-primary" data-testid="add-policy" :disabled="saving || needsReload || availableGroups.length === 0" @click="openEditor()">{{ t('admin.clientAccess.addPolicy') }}</button>
          </div>
          <p v-if="groups.length === 0" class="text-sm text-gray-500">{{ t('admin.clientAccess.noGroups') }}</p>
          <p v-else-if="availableGroups.length === 0" class="text-sm text-gray-500">{{ t('admin.clientAccess.allConfigured') }}</p>
          <label v-if="policies.length" class="block">{{ t('admin.clientAccess.searchGroup') }}<input v-model="search" class="input mt-1" type="search" /></label>
          <p v-if="policies.length === 0" class="rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-800">{{ t('admin.clientAccess.noPolicies') }}</p>
          <p v-else-if="filteredPolicies.length === 0" class="text-sm text-gray-500">{{ t('admin.clientAccess.noMatches') }}</p>
          <ul v-else class="divide-y divide-gray-200 dark:divide-dark-600">
            <li v-for="entry in filteredPolicies" :key="entry.id" class="flex flex-wrap items-center justify-between gap-3 py-4" :data-policy-id="entry.id">
              <div class="min-w-0 space-y-1"><h3 class="font-medium break-words">{{ entry.name }} <span class="text-xs text-gray-500">ID {{ entry.id }}</span></h3><p class="text-sm">{{ t(`admin.clientAccess.${entry.policy.mode}`) }}</p><p class="text-sm text-gray-500 break-words">{{ policyPreview(entry.policy, config.clients) }}</p><ClientPolicyDates :policy="entry.policy" /></div>
              <div class="flex flex-wrap gap-2"><button type="button" class="btn btn-secondary" :disabled="saving || needsReload || !entry.exists" @click="openEditor(entry.id)">{{ t('common.edit') }}</button><button type="button" class="btn btn-secondary" :disabled="saving || needsReload" @click="removePolicy(entry.id, entry.name)">{{ t('admin.clientAccess.removePolicy') }}</button><button type="button" class="btn btn-secondary" data-testid="policy-logs" @click="logGroup = { id: entry.id, name: entry.name }">{{ t('admin.clientAccess.logs') }}</button></div>
            </li>
          </ul>
        </section>
        <ClientRegistryEditor v-else ref="registry" :config="config" :groups="groups" :saving="saving || needsReload" @save="saveClients" />
        <BaseDialog :show="editorOpen" :title="t(adding ? 'admin.clientAccess.addPolicy' : 'admin.clientAccess.editPolicy')" :show-close-button="!saving" :close-on-escape="!saving" @close="closeEditor">
          <p v-if="error" role="alert" class="mb-4 text-sm text-red-600">{{ error }}</p>
          <GroupClientPolicyEditor v-model="draft" :clients="config.clients" :saving="saving || needsReload" :dirty="policyDirty" :can-save="!!selectedGroup" :restricted-only="adding" cancelable @save="savePolicy" @cancel="closeEditor">
            <label v-if="adding" class="block">{{ t('admin.clientAccess.selectGroup') }}
              <select v-model="selectedGroup" class="input mt-1" required data-testid="new-policy-group"><option value="">{{ t('admin.clientAccess.selectGroup') }}</option><option v-for="group in availableGroups" :key="group.id" :value="String(group.id)">{{ group.name }} · ID {{ group.id }}</option></select>
            </label>
            <p v-else class="font-medium">{{ currentGroup?.name }} · ID {{ selectedGroup }}</p>
          </GroupClientPolicyEditor>
        </BaseDialog>
      </template>
    </template>
  <ClientPolicyDenialLogs v-if="logGroup" :group-id="logGroup.id" :group-name="logGroup.name" @close="logGroup = null" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getAllIncludingInactive } from '@/api/admin/groups'
import { getClientAccess, saveClientAccess, policyPreview } from '@/api/admin/clientAccess'
import type { ClientAccessConfig, ClientDefinition, GroupClientPolicy } from '@/api/admin/clientAccess'
import type { AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ClientRegistryEditor from './ClientRegistryEditor.vue'
import GroupClientPolicyEditor from './GroupClientPolicyEditor.vue'
import ClientPolicyDates from './ClientPolicyDates.vue'
import ClientPolicyDenialLogs from './ClientPolicyDenialLogs.vue'

const props = defineProps<{ groupId?: string; initialTab?: string }>()
const { t } = useI18n()
const app = useAppStore()
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value))
const off = (): GroupClientPolicy => ({ mode: 'off', client_ids: [] })
const config = ref<ClientAccessConfig | null>(null)
const groups = ref<AdminGroup[]>([])
const selectedGroup = ref(props.groupId ?? '')
const draft = ref<GroupClientPolicy>(off())
const draftBaseline = ref(JSON.stringify(off()))
const activeTab = ref<'policies' | 'clients'>(props.initialTab === 'clients' ? 'clients' : 'policies')
const registry = ref<InstanceType<typeof ClientRegistryEditor> | null>(null)
const logGroup = ref<{ id: string; name: string } | null>(null)
const search = ref('')
const loading = ref(true)
const saving = ref(false)
const needsReload = ref(false)
const error = ref('')
const editorOpen = ref(false)
const adding = ref(false)
const scoped = computed(() => props.groupId !== undefined)
const currentGroup = computed(() => groups.value.find(group => String(group.id) === selectedGroup.value))
const groupError = computed(() => !loading.value && config.value && scoped.value && !currentGroup.value ? t('admin.clientAccess.groupUnavailable', { id: props.groupId }) : '')
const policyDirty = computed(() => JSON.stringify(draft.value) !== draftBaseline.value || (adding.value && editorOpen.value && !!selectedGroup.value))
const dirty = computed(() => !!registry.value?.dirty || ((scoped.value || editorOpen.value) && policyDirty.value))
const policies = computed(() => Object.entries(config.value?.groups ?? {}).filter(([, policy]) => policy.mode !== 'off').map(([id, policy]) => {
  const group = groups.value.find(group => String(group.id) === id)
  return { id, policy, name: group?.name ?? t('admin.clientAccess.groupUnavailable', { id }), exists: !!group }
}))
const filteredPolicies = computed(() => policies.value.filter(entry => `${entry.id} ${entry.name}`.toLowerCase().includes(search.value.toLowerCase())))
const availableGroups = computed(() => groups.value.filter(group => !config.value?.groups[String(group.id)] || config.value.groups[String(group.id)].mode === 'off'))
function switchTab(tab: 'policies' | 'clients') {
  if (activeTab.value === tab || (dirty.value && !window.confirm(t('admin.clientAccess.discard')))) return
  activeTab.value = tab
  if (!needsReload.value) error.value = ''
}
function openEditor(id?: string) {
  selectedGroup.value = id ?? ''
  adding.value = id === undefined
  draft.value = id ? clone(config.value?.groups[id] ?? off()) : { mode: 'allow', client_ids: [] }
  draftBaseline.value = JSON.stringify(draft.value)
  error.value = ''
  editorOpen.value = true
}
function closeEditor() {
  if (saving.value || (policyDirty.value && !window.confirm(t('admin.clientAccess.discard')))) return
  editorOpen.value = false
  if (!needsReload.value) error.value = ''
}
async function reload() {
  if (dirty.value && !window.confirm(t('admin.clientAccess.discard'))) return
  loading.value = true
  error.value = ''
  try {
    const [next, liveGroups] = await Promise.all([getClientAccess(), getAllIncludingInactive()])
    config.value = next
    groups.value = liveGroups
    needsReload.value = false
    editorOpen.value = false
    selectedGroup.value = props.groupId ?? ''
    draft.value = clone(next.groups[selectedGroup.value] ?? off())
    draftBaseline.value = JSON.stringify(draft.value)
  } catch { error.value = t('admin.clientAccess.loadError'); config.value = null }
  finally { loading.value = false }
}
async function persist(next: ClientAccessConfig): Promise<boolean> {
  if (saving.value || needsReload.value) return false
  saving.value = true
  error.value = ''
  let written = false
  try {
    await saveClientAccess(next)
    written = true
    // 2026-10-14 coder(lq): 仍复用文档级 CAS，但只替换当前保存范围；回读失败锁定写入，防止复用旧 revision。
    config.value = await getClientAccess()
    app.showSuccess(t('admin.clientAccess.saved'))
    return true
  } catch (err: unknown) {
    const failure = err as { status?: number; message?: string }
    needsReload.value = written
    error.value = written ? t('admin.clientAccess.readbackError') : failure.status === 409 ? t('admin.clientAccess.conflict') : failure.message || t('admin.clientAccess.saveError')
    return false
  } finally { saving.value = false }
}
async function savePolicy() {
  if (!config.value || !currentGroup.value || (adding.value && !availableGroups.value.some(group => String(group.id) === selectedGroup.value))) return
  if (draft.value.mode !== 'off' && (!draft.value.client_ids.length || draft.value.client_ids.some(id => !config.value?.clients.some(client => client.id === id && client.enabled)))) { error.value = t('admin.clientAccess.required'); return }
  const next = clone(config.value)
  if (draft.value.mode === 'off') delete next.groups[selectedGroup.value]
  else next.groups[selectedGroup.value] = clone(draft.value)
  if (await persist(next)) {
    draft.value = clone(config.value.groups[selectedGroup.value] ?? off())
    draftBaseline.value = JSON.stringify(draft.value)
    editorOpen.value = false
  }
}
async function removePolicy(id: string, name: string) {
  if (!config.value || !window.confirm(t('admin.clientAccess.removePolicyConfirm', { group: name }))) return
  const next = clone(config.value)
  delete next.groups[id]
  await persist(next)
}
async function saveClients(clients: ClientDefinition[]) {
  if (!config.value) return
  if (new Set(clients.map(client => client.id)).size !== clients.length || clients.some(client => !/^[a-z][a-z0-9_-]{0,63}$/.test(client.id) || !client.name.trim() || (client.detector === 'headers' && (!client.rules.length || client.rules.some(rule => !rule.header.trim() || !rule.value.trim()))))) { error.value = t('admin.clientAccess.invalidClient'); return }
  const next = clone(config.value)
  next.clients = clone(clients)
  await persist(next)
}
onMounted(reload)
defineExpose({ dirty })
</script>
