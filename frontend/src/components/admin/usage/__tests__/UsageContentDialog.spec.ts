import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UsageContentDialog from '../UsageContentDialog.vue'
import type { UsageContentDetail, UsageContentResponse } from '@/api/admin/usageContent'

const get = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin/usageContent', () => ({ getUsageContent: get }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))

const record = (id: number) => ({ id, model: 'test-model', request_id: `request-${id}`, created_at: new Date().toISOString() })
const detail = (overrides: Partial<UsageContentDetail> = {}): UsageContentDetail => ({
  capture_id: 'capture-1', request_id: 'request-1', client_request_id: 'internal-client-id', created_at: new Date().toISOString(),
  expires_at: new Date(Date.now() + 3_600_000).toISOString(), method: 'POST', path: '/v1/responses', protocol: 'http', status: 200,
  partial: false, request_content_type: 'application/json', response_content_type: 'text/event-stream',
  request_bytes: 100, response_bytes: 100, request_stored_bytes: 100, response_stored_bytes: 100, request_truncated: false, response_truncated: false,
  request_body: '{"instructions":"system prompt","input":"private question"}',
  response_body: 'data: {"type":"response.output_text.delta","delta":"private answer"}\n\n', ...overrides
})
const response = (content: UsageContentDetail | null, emptyReason: UsageContentResponse['empty_reason'] = ''): UsageContentResponse => ({
  content, captures: content ? [content] : [], enabled: true, retention_hours: 24, empty_reason: emptyReason,
  stats: { dropped: 0, write_failures: 0, cleanup_failures: 0, truncated: 0 }
})
const options = { global: { stubs: { BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /></section>' } } } }

describe('Admin request content dialog', () => {
  beforeEach(() => get.mockReset().mockResolvedValue(response(detail())))
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  it('does not fetch before opening and loads readable input and streaming output on demand', async () => {
    const wrapper = mount(UsageContentDialog, { props: { record: null }, ...options })
    await flushPromises(); expect(get).not.toHaveBeenCalled()
    await wrapper.setProps({ record: record(1) }); await flushPromises()
    expect(wrapper.text()).toContain('system prompt')
    expect(wrapper.text()).toContain('private question')
    expect(wrapper.text()).toContain('private answer')
    expect(get.mock.calls[0][1].signal).toBeInstanceOf(AbortSignal)
    wrapper.unmount()
  })
  it('clears bodies and cancels pending reads when closing or changing records', async () => {
    let resolveOld!: (value: UsageContentResponse) => void
    get.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    const signal = get.mock.calls[0][1].signal as AbortSignal
    await wrapper.setProps({ record: record(2) }); await flushPromises()
    expect(signal.aborted).toBe(true)
    resolveOld(response(detail({ request_body: '{"input":"stale secret"}' }))); await flushPromises()
    expect(wrapper.text()).not.toContain('stale secret')
    await wrapper.setProps({ record: null }); await flushPromises()
    expect(wrapper.text()).not.toContain('private question')
    expect(wrapper.findAll('textarea')).toHaveLength(0)
    wrapper.unmount()
  })
  it('removes an already-open payload at its absolute expiry', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
    vi.setSystemTime(new Date('2026-11-10T08:00:00Z'))
    get.mockResolvedValue(response(detail({ expires_at: new Date(Date.now() + 1000).toISOString() })))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises(); expect(wrapper.text()).toContain('private question')
    await vi.advanceTimersByTimeAsync(1100)
    expect(wrapper.text()).toContain('usageContent.expired')
    expect(wrapper.text()).not.toContain('private question')
    expect(wrapper.text()).not.toContain('private answer')
    wrapper.unmount()
  })
  it('offers a safe raw view without executing user HTML', async () => {
    get.mockResolvedValue(response(detail({ request_body: '{"input":"<img src=x onerror=alert(1)>"}' })))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises(); expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toContain('<img src=x onerror=alert(1)>')
    await wrapper.findAll('button').find(button => button.text() === 'usageContent.raw')!.trigger('click')
    const raw = wrapper.findAll('textarea')
    expect(raw).toHaveLength(2)
    expect(raw[0].attributes()).toHaveProperty('readonly')
    expect((raw[0].element as HTMLTextAreaElement).value).toContain('<img src=x onerror=alert(1)>')
    expect((raw[1].element as HTMLTextAreaElement).value).toContain('data: ')
    wrapper.unmount()
  })
  it('marks truncation and partial output instead of presenting it as complete', async () => {
    get.mockResolvedValue(response(detail({ request_truncated: true, response_truncated: true, partial: true })))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises()
    expect(wrapper.text().match(/usageContent.truncated/g)).toHaveLength(2)
    expect(wrapper.text()).toContain('usageContent.partial')
    wrapper.unmount()
  })
  it('uses the error request endpoint and can retry a failed load', async () => {
    get.mockRejectedValueOnce(new Error('temporary'))
    const wrapper = mount(UsageContentDialog, { props: { record: record(9), errorRecord: true }, ...options })
    await flushPromises(); expect(wrapper.text()).toContain('usageContent.loadError')
    expect(get.mock.calls[0][1].errorRecord).toBe(true)
    await wrapper.findAll('button').find(button => button.text() === 'usageContent.retry')!.trigger('click')
    await flushPromises(); expect(wrapper.text()).toContain('private answer')
    wrapper.unmount()
  })
  it('explains expired and historical missing data without inventing content', async () => {
    get.mockResolvedValue(response(null, 'expired'))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises(); expect(wrapper.text()).toContain('usageContent.expired')
    expect(wrapper.findAll('textarea')).toHaveLength(0)
    get.mockResolvedValue(response(null, 'not_recorded'))
    await wrapper.setProps({ record: record(2) }); await flushPromises()
    expect(wrapper.text()).toContain('usageContent.empty')
    wrapper.unmount()
  })
  it('limits initial message rendering and expands history on demand', async () => {
    get.mockResolvedValue(response(detail({ request_body: JSON.stringify({ messages: Array.from({ length: 80 }, (_, i) => ({ role: 'user', content: `history-${i}` })) }) })))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(50)
    await wrapper.findAll('button').find(button => button.text() === 'usageContent.showMore')!.trigger('click')
    expect(wrapper.findAll('article')).toHaveLength(80)
    wrapper.unmount()
  })
  it('falls back to original text for large bodies without workers', async () => {
    vi.stubGlobal('Worker', undefined)
    const raw = JSON.stringify({ input: 'large private input '.repeat(10000) })
    get.mockResolvedValue(response(detail({ request_body: raw })))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(0)
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe(raw)
    wrapper.unmount()
  })
  it('terminates large-body workers and rejects late results after closing', async () => {
    const workers: Array<{ onmessage: ((event: { data: unknown }) => void) | null; onerror: (() => void) | null; terminate: ReturnType<typeof vi.fn>; postMessage: ReturnType<typeof vi.fn> }> = []
    class FakeWorker {
      onmessage = null
      onerror = null
      terminate = vi.fn()
      postMessage = vi.fn()
      constructor() { workers.push(this) }
    }
    vi.stubGlobal('Worker', FakeWorker)
    get.mockResolvedValue(response(detail({ request_body: JSON.stringify({ input: 'x'.repeat(150000) }) })))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises()
    expect(workers).toHaveLength(1)
    expect(workers[0].postMessage).toHaveBeenCalledWith(expect.objectContaining({ mode: 'conversation' }))
    expect(wrapper.text()).toContain('usageContent.parsing')
    await wrapper.setProps({ record: null })
    expect(workers[0].terminate).toHaveBeenCalled()
    workers[0].onmessage?.({ data: { messages: [{ role: 'user', text: 'late secret' }], output: { text: '', reasoning: '', tools: [] }, rawRequest: '', rawResponse: '' } })
    await flushPromises()
    expect(wrapper.text()).not.toContain('late secret')
    wrapper.unmount()
  })
  it('renders a worker result and reparses only the selected mode', async () => {
    const workers: Array<{ onmessage: ((event: { data: unknown }) => void) | null; onerror: (() => void) | null; terminate: ReturnType<typeof vi.fn>; postMessage: ReturnType<typeof vi.fn> }> = []
    const current = () => workers[workers.length - 1]
    class FakeWorker {
      onmessage = null
      onerror = null
      terminate = vi.fn()
      postMessage = vi.fn()
      constructor() { workers.push(this) }
    }
    vi.stubGlobal('Worker', FakeWorker)
    get.mockResolvedValue(response(detail({ request_body: JSON.stringify({ input: 'x'.repeat(150000) }) })))
    const wrapper = mount(UsageContentDialog, { props: { record: record(1) }, ...options })
    await flushPromises()
    current().onmessage?.({ data: { messages: [{ role: 'user', text: 'worker question' }], output: { text: 'worker answer', reasoning: '', tools: [] }, rawRequest: '', rawResponse: '' } })
    await flushPromises()
    expect(wrapper.text()).toContain('worker answer')
    expect(current().terminate).toHaveBeenCalled()
    await wrapper.findAll('button').find(button => button.text() === 'usageContent.raw')!.trigger('click')
    expect(current().postMessage).toHaveBeenCalledWith(expect.objectContaining({ mode: 'raw' }))
    current().onerror?.()
    await flushPromises()
    expect(wrapper.findAll('textarea')).toHaveLength(2)
    wrapper.unmount()
  })
})
