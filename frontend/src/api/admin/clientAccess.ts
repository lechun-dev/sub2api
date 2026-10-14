import { apiClient } from '../client'

export interface ClientHeaderRule {
  header: string
  operator: 'exact' | 'prefix' | 'contains'
  value: string
}
export interface ClientDefinition {
  id: string
  name: string
  enabled: boolean
  detector: 'codex' | 'headers'
  match: 'all' | 'any'
  rules: ClientHeaderRule[]
}
export interface GroupClientPolicy {
  mode: 'off' | 'allow' | 'deny'
  client_ids: string[]
}
export interface ClientAccessConfig {
  revision: string
  clients: ClientDefinition[]
  groups: Record<string, GroupClientPolicy>
}
export async function getClientAccess(): Promise<ClientAccessConfig> {
  return (await apiClient.get<ClientAccessConfig>('/admin/client-access')).data
}
export async function saveClientAccess(config: ClientAccessConfig): Promise<void> {
  await apiClient.put('/admin/client-access', config)
}

export function referencingGroups(config: ClientAccessConfig, clientId: string): string[] {
  return Object.entries(config.groups)
    .filter(([, policy]) => policy.mode !== 'off' && policy.client_ids.includes(clientId))
    .map(([id]) => id)
}

export function policyPreview(policy: GroupClientPolicy, clients: ClientDefinition[]): string {
  return policy.client_ids.map(id => clients.find(client => client.id === id)?.name ?? id).join('、')
}
