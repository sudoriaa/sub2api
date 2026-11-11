import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UsageTable from '../UsageTable.vue'
import { observerUsageContext } from '../observerUsageContext'
import type { AdminUsageLog } from '@/types'

const get = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin/usageContent', () => ({ getUsageContent: get }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: {} }) }))

const row = { id: 42, model: 'model', request_id: 'request-42', created_at: new Date().toISOString() } as AdminUsageLog
const stubs = {
  DataTable: { props: ['data'], template: '<div><slot v-for="row in data" name="cell-content" :row="row" /></div>' },
  UsageTimingDialog: true,
  BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /></section>' }
}

describe('Admin usage table dialogue entry', () => {
  beforeEach(() => get.mockReset().mockResolvedValue({ content: null, captures: [], enabled: true, retention_hours: 24, empty_reason: 'not_recorded', stats: {} }))
  it('does not fetch bodies on list rendering, only after an admin click', async () => {
    const wrapper = mount(UsageTable, { props: { data: [row], columns: [{ key: 'content', label: 'content' }], enableContentDetails: true }, global: { stubs } })
    await flushPromises(); expect(get).not.toHaveBeenCalled()
    await wrapper.find('[data-testid="usage-content-button"]').trigger('click'); await flushPromises()
    expect(get).toHaveBeenCalledTimes(1)
    expect(get.mock.calls[0][0]).toBe(42)
    wrapper.unmount()
  })
  it('never exposes or fetches dialogue in user or observer tables', async () => {
    for (const observer of [false, true]) {
      const wrapper = mount(UsageTable, {
        props: { data: [row], columns: [{ key: 'content', label: 'content' }], enableContentDetails: observer },
        global: { stubs, provide: { [observerUsageContext as symbol]: observer } }
      })
      await flushPromises(); expect(wrapper.find('[data-testid="usage-content-button"]').exists()).toBe(false)
      wrapper.unmount()
    }
    expect(get).not.toHaveBeenCalled()
  })
})
