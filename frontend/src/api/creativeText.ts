import { buildGatewayUrl } from './client'

export interface CreativeTextMessage { role: 'system' | 'user' | 'assistant'; content: string }

async function checkResponse(response: Response) {
  if (response.ok) return
  const body = await response.json().catch(() => null)
  const code = body?.error?.code || body?.error?.type || body?.code
  const message = body?.error?.message || body?.message
  if (code === 'INSUFFICIENT_BALANCE') {
    throw new Error('账户余额不足，暂时无法生成文本。请充值后重试，输入内容已保留。')
  }
  if (message === 'All available accounts exhausted') {
    if (response.status === 401) {
      throw new Error('当前渠道的上游账号认证失败，暂时无法生成。请联系管理员检查上游凭证，或切换其他渠道。')
    }
    throw new Error('当前渠道暂无可用的上游账号，请稍后重试或切换其他渠道。')
  }
  if (message === 'Service temporarily unavailable') {
    throw new Error('当前渠道暂时不可用，请稍后重试或切换渠道；管理员可检查分组是否绑定了可用账号。')
  }
  if (message === 'Upstream access forbidden, please contact administrator') {
    throw new Error('上游服务拒绝访问，请联系管理员检查上游接口地址和访问限制，或切换其他渠道。')
  }
  throw new Error(message || `请求失败 (${response.status})`)
}

export async function listCreativeTextModels(key: string, signal?: AbortSignal): Promise<string[]> {
  const response = await fetch(buildGatewayUrl('/v1/models'), { headers: { Authorization: `Bearer ${key}` }, signal })
  await checkResponse(response)
  const body = await response.json()
  return [...new Set<string>((body.data || []).map((m: { id: string }) => m.id).filter(Boolean))]
}

// Parse SSE frames across arbitrary network chunks, including CRLF and a final
// frame without a trailing blank line. Provider error frames must stay visible.
export async function streamCreativeText(key: string, model: string, messages: CreativeTextMessage[], signal: AbortSignal, onText: (text: string) => void) {
  const response = await fetch(buildGatewayUrl('/v1/chat/completions'), {
    method: 'POST', headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ model, messages, stream: true }), signal,
  })
  await checkResponse(response)
  if (!response.headers.get('content-type')?.includes('text/event-stream')) {
    const body = await response.json()
    if (body.error) throw new Error(body.error.message || '生成失败')
    const text = body.choices?.[0]?.message?.content
    if (typeof text !== 'string') throw new Error('未收到文本内容')
    onText(text)
    return
  }
  if (!response.body) throw new Error('未收到响应')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  function frame(value: string) {
    const data = value.split(/\r?\n/).filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart()).join('\n')
    if (data === '[DONE]') return true
    if (!data) return false
    const event = JSON.parse(data)
    if (event.error) throw new Error(event.error.message || '生成失败')
    const delta = event.choices?.[0]?.delta?.content
    if (typeof delta === 'string') onText(delta)
  }
  try {
    while (true) {
      const { done, value } = await reader.read()
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true })
      let match: RegExpExecArray | null
      while ((match = /\r?\n\r?\n/.exec(buffer))) {
        if (frame(buffer.slice(0, match.index))) return
        buffer = buffer.slice(match.index + match[0].length)
      }
      if (done) { if (buffer.trim()) frame(buffer); break }
    }
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock() }
}
