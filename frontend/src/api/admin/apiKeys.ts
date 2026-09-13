/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey, PaginatedResponse, UpdateApiKeyRequest } from '@/types'

export interface AdminAPIKeyListFilters {
  search?: string
  status?: string
  group_id?: number
}

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

/** Update an API key's editable fields with administrator privileges. */
export async function updateAPIKey(id: number, request: UpdateApiKeyRequest): Promise<ApiKey> {
  const { data } = await apiClient.patch<{ api_key: ApiKey }>(`/admin/api-keys/${id}`, request)
  return data.api_key
}

/** Delete an API key with administrator privileges. */
export async function deleteAPIKey(id: number): Promise<void> {
  await apiClient.delete(`/admin/api-keys/${id}`)
}

/** List every non-deleted API key, grouped by owner in the server response. */
export async function listAdminAPIKeys(
  page = 1,
  pageSize = 20,
  filters?: AdminAPIKeyListFilters
): Promise<PaginatedResponse<ApiKey>> {
  const { data } = await apiClient.get<PaginatedResponse<ApiKey>>('/admin/api-keys', {
    params: {
      page,
      page_size: pageSize,
      search: filters?.search,
      status: filters?.status,
      group_id: filters?.group_id
    }
  })
  return data
}

export const apiKeysAPI = {
  issueAPIKey,
  updateApiKeyGroup,
  updateAPIKey,
  deleteAPIKey,
  listAdminAPIKeys
}

export default apiKeysAPI
