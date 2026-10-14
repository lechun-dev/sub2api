import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

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
  created_at?: string | null
  updated_at?: string | null
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

export interface ClientPolicySnapshot {
  code: 'client_not_allowed' | 'client_unrecognized' | 'client_forbidden'
  mode: 'allow' | 'deny'
  detected_clients: { id: string; name: string }[]
  selected_clients: { id: string; name: string }[]
  detected_count: number
  selected_count: number
  policy_updated_at?: string
}
export interface ClientPolicyDenialLog {
  id: number
  created_at: string
  group_id: number
  username: string
  user_id?: number
  api_key_id?: number
  client_ip: string
  request_id: string
  request_path: string
  snapshot?: ClientPolicySnapshot
}
export async function getClientPolicyDenials(params: {
  group_id: string; start_time: string; end_time: string; page: number; page_size: number
}, signal?: AbortSignal): Promise<PaginatedResponse<ClientPolicyDenialLog>> {
  return (await apiClient.get<PaginatedResponse<ClientPolicyDenialLog>>('/admin/ops/client-policy-denials', { params, signal })).data
}
