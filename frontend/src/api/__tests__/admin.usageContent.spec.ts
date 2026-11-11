import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getUsageContent } from '../admin/usageContent'

const get = vi.hoisted(() => vi.fn())
vi.mock('../client', () => ({ apiClient: { get } }))

describe('Admin request content API', () => {
  beforeEach(() => get.mockReset().mockResolvedValue({ data: { content: null, captures: [] } }))
  it('uses admin-only detail endpoints and passes abort and capture selection', async () => {
    const signal = new AbortController().signal
    await getUsageContent(42, { signal, captureId: 'selected-capture' })
    expect(get).toHaveBeenCalledWith('/admin/usage/42/content', { signal, params: { capture_id: 'selected-capture' } })
    await getUsageContent(43, { signal, errorRecord: true })
    expect(get).toHaveBeenLastCalledWith('/admin/usage/errors/43/content', { signal, params: undefined })
  })
})
