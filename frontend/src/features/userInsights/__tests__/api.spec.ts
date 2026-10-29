import { describe, expect, it, vi } from 'vitest'
import { userInsightsAPI } from '../api'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

describe('userInsightsAPI', () => {
  it('sends sorting and pagination to the server', async () => {
    const response = { items: [], total: 0 }
    get.mockResolvedValue({ data: response })
    const signal = new AbortController().signal
    expect(await userInsightsAPI.list(2, 'alice', 'inactive30', 'total_calls', 'asc', signal)).toBe(response)
    expect(get).toHaveBeenCalledWith('/admin/user-insights', {
      params: { page: 2, page_size: 20, search: 'alice', segment: 'inactive30', sort_by: 'total_calls', sort_order: 'asc' },
      signal
    })
  })
})
