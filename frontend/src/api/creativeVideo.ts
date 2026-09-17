import { buildGatewayUrl } from './client'

export type CreativeVideoStatus =
  | 'queued'
  | 'submitted'
  | 'running'
  | 'completed'
  | 'failed'
  | 'expired'
  | 'output_deleted'
  | string

export interface CreativeVideoTask {
  id: string
  object: string
  status: CreativeVideoStatus
  provider: string
  model: string
  prompt_preview?: string | null
  resolution?: string | null
  duration_seconds?: number | null
  created_at: number
  submitted_at?: number | null
  completed_at?: number | null
  downloaded_at?: number | null
  output_deleted_at?: number | null
}

export interface CreativeVideoTasksResponse {
  object: string
  data: CreativeVideoTask[]
  has_more: boolean
  retention_days: number
  max_records_per_user: number
  max_running_per_user: number
}

export interface CreativeVideoCreateRequest {
  model: string
  prompt: string
  aspect_ratio?: string
  resolution?: string
  duration?: number
  image?: {
    type?: 'image_url' | string
    url: string
  }
}

async function parseError(response: Response): Promise<Error> {
  try {
    const body = await response.json()
    const error = new Error(body?.error?.message || body?.message || response.statusText)
    ;(error as any).code = body?.error?.code || response.status
    ;(error as any).status = response.status
    return error
  } catch {
    const error = new Error(response.statusText || `HTTP ${response.status}`)
    ;(error as any).code = response.status
    ;(error as any).status = response.status
    return error
  }
}

function headers(apiKey: string, extra?: HeadersInit): HeadersInit {
  return {
    Authorization: `Bearer ${apiKey}`,
    ...extra,
  }
}

function creativeHeaders(): HeadersInit {
  return { 'X-Sub2API-Creative-Workbench': 'video' }
}

export async function createCreativeVideo(apiKey: string, payload: CreativeVideoCreateRequest): Promise<any> {
  const response = await fetch(buildGatewayUrl('/v1/videos/generations'), {
    method: 'POST',
    headers: headers(apiKey, {
      'Content-Type': 'application/json',
      ...creativeHeaders(),
    }),
    body: JSON.stringify(payload),
  })
  if (!response.ok) throw await parseError(response)
  return response.json()
}

export async function getCreativeVideoStatus(apiKey: string, requestId: string): Promise<any> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/${encodeURIComponent(requestId)}`), {
    headers: headers(apiKey, creativeHeaders()),
  })
  if (!response.ok) throw await parseError(response)
  return response.json()
}

export async function listCreativeVideoTasks(apiKey: string, limit = 50): Promise<CreativeVideoTasksResponse> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/studio-tasks?limit=${encodeURIComponent(String(limit))}`), {
    headers: headers(apiKey),
  })
  if (!response.ok) throw await parseError(response)
  return response.json()
}

export async function downloadCreativeVideo(apiKey: string, requestId: string): Promise<Blob> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/${encodeURIComponent(requestId)}/content`), {
    headers: headers(apiKey, creativeHeaders()),
  })
  if (!response.ok) throw await parseError(response)
  const blob = await response.blob()
  const contentType = response.headers.get('Content-Type') || blob.type || 'video/mp4'
  if (blob.type === contentType) return blob
  return new Blob([blob], { type: contentType.includes('video/') ? contentType : 'video/mp4' })
}

export async function deleteCreativeVideoTask(apiKey: string, requestId: string): Promise<void> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/studio-tasks/${encodeURIComponent(requestId)}`), {
    method: 'DELETE',
    headers: headers(apiKey),
  })
  if (!response.ok) throw await parseError(response)
}
