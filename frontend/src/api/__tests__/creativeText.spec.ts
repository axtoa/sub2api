import { afterEach, describe, expect, it, vi } from 'vitest'
import { streamCreativeText } from '../creativeText'

vi.mock('../client', () => ({ buildGatewayUrl: (path: string) => path }))
afterEach(() => vi.unstubAllGlobals())

describe('creative text streaming', () => {
  it.each([
    { status: 503, upstream: 'Service temporarily unavailable', expected: '当前渠道暂时不可用' },
    { status: 502, upstream: 'Upstream access forbidden, please contact administrator', expected: '上游服务拒绝访问' },
  ])('explains $upstream without blaming the user balance', async ({ status, upstream, expected }) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { type: 'api_error', message: upstream } }), { status, headers: { 'Content-Type': 'application/json' } })))
    await expect(streamCreativeText('key', 'model', [], new AbortController().signal, () => {})).rejects.toThrow(expected)
  })
  it.each([
    [401, '当前渠道的上游账号认证失败'],
    [502, '当前渠道暂无可用的上游账号'],
  ])('explains exhausted upstream accounts for HTTP %s', async (status, message) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { type: 'server_error', message: 'All available accounts exhausted' } }), { status: status as number, headers: { 'Content-Type': 'application/json' } })))
    await expect(streamCreativeText('key', 'model', [], new AbortController().signal, () => {})).rejects.toThrow(message as string)
  })
  it('explains a local insufficient balance rejection in Chinese', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: 'INSUFFICIENT_BALANCE', message: 'Insufficient account balance' }), { status: 403, headers: { 'Content-Type': 'application/json' } })))
    await expect(streamCreativeText('key', 'model', [], new AbortController().signal, () => {})).rejects.toThrow('账户余额不足，暂时无法生成文本。请充值后重试，输入内容已保留。')
  })
  it('joins split UTF-8 and CRLF frames and uses the existing gateway', async () => {
    const bytes = new TextEncoder().encode('data: {"choices":[{"delta":{"content":"你好"}}]}\r\n\r\ndata: [DONE]\n\n')
    const stream = new ReadableStream({ start(controller) { for (const byte of bytes) controller.enqueue(new Uint8Array([byte])); controller.close() } })
    const fetch = vi.fn().mockResolvedValue(new Response(stream, { headers: { 'Content-Type': 'text/event-stream' } }))
    vi.stubGlobal('fetch', fetch)
    let text = ''
    await streamCreativeText('test-key', 'model', [{ role: 'user', content: 'hello' }], new AbortController().signal, delta => text += delta)
    expect(text).toBe('你好')
    expect(fetch.mock.calls[0][0]).toBe('/v1/chat/completions')
    expect(JSON.parse(fetch.mock.calls[0][1].body).messages).toEqual([{ role: 'user', content: 'hello' }])
  })
  it('surfaces upstream error events instead of silently succeeding', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('data: {"error":{"message":"quota exhausted"}}\n\n', { headers: { 'Content-Type': 'text/event-stream' } })))
    await expect(streamCreativeText('key', 'model', [], new AbortController().signal, () => {})).rejects.toThrow('quota exhausted')
  })
  it('accepts compatible non-streaming replies', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ choices: [{ message: { content: 'answer' } }] }), { headers: { 'Content-Type': 'application/json' } })))
    const onText = vi.fn()
    await streamCreativeText('key', 'model', [], new AbortController().signal, onText)
    expect(onText).toHaveBeenCalledWith('answer')
  })
})
