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
  afterEach(() => vi.useRealTimers())
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
})
