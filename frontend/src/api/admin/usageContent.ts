import { apiClient } from '../client'

export interface UsageContentRecord {
  id: number
  model: string
  request_id: string
  created_at: string
}

export interface UsageContentReference {
  capture_id: string
  request_id: string
  client_request_id: string
  created_at: string
  expires_at: string
  method: string
  path: string
  protocol: string
  status: number
  partial: boolean
  request_content_type: string
  response_content_type: string
  request_bytes: number
  response_bytes: number
  request_stored_bytes: number
  response_stored_bytes: number
  request_truncated: boolean
  response_truncated: boolean
  request_omitted?: string
  response_omitted?: string
}

export interface UsageContentDetail extends UsageContentReference {
  request_body: string
  response_body: string
}

export interface UsageContentResponse {
  content: UsageContentDetail | null
  captures: UsageContentReference[]
  retention_hours: number
  enabled: boolean
  empty_reason: '' | 'expired' | 'not_recorded' | 'capacity'
  stats: { dropped: number; write_failures: number; cleanup_failures: number; truncated: number }
}

// Payloads are intentionally loaded only on a user click, never with a list,
// export, prefetch, persisted store or dashboard refresh.
export async function getUsageContent(
  id: number,
  options: { signal?: AbortSignal; captureId?: string; errorRecord?: boolean } = {}
): Promise<UsageContentResponse> {
  const path = options.errorRecord ? `/admin/usage/errors/${id}/content` : `/admin/usage/${id}/content`
  const { data } = await apiClient.get<UsageContentResponse>(path, {
    signal: options.signal,
    params: options.captureId ? { capture_id: options.captureId } : undefined
  })
  return data
}
