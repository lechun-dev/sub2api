/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey } from '@/types'

export interface IssueAPIKeyRequest {
  user_id: number
  name: string
  group_id?: number | null
  quota?: number
  expires_in_days?: number | null
  rate_limit_5h?: number
  rate_limit_1d?: number
  rate_limit_7d?: number
  ip_whitelist?: string[]
  ip_blacklist?: string[]
}

export interface IssueAPIKeyResponse {
  api_key: ApiKey
  plaintext_key: string
}

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

/**
 * Issue an API key for an existing user without requiring user login.
 */
export async function issueAPIKey(
  request: IssueAPIKeyRequest,
  idempotencyKey: string
): Promise<IssueAPIKeyResponse> {
  const { data } = await apiClient.post<IssueAPIKeyResponse>('/admin/api-keys/issue', request, {
    headers: { 'Idempotency-Key': idempotencyKey }
  })
  return data
}

/**
 * Update an API key's group binding
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(id: number, groupId: number | null): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, {
    group_id: groupId === null ? 0 : groupId
  })
  return data
}

export const apiKeysAPI = {
  issueAPIKey,
  updateApiKeyGroup
}

export default apiKeysAPI
