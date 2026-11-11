<template>
  <BaseDialog :show="!!record" :title="t('usageContent.title')" width="extra-wide"
    :placement="viewportWidth < 768 ? 'center' : 'right'" close-on-click-outside @close="close">
    <div v-if="record" class="space-y-4">
      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
        <div class="flex flex-wrap justify-between gap-2"><strong>{{ record.model }}</strong><span>{{ formatDateTime(record.created_at) }}</span></div>
        <div class="mt-2 break-all font-mono text-xs text-gray-500">{{ record.request_id }}</div>
      </div>
      <p class="text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ t('usageContent.retentionNote') }}</p>
      <p v-if="loading" role="status">{{ t('usageContent.loading') }}</p>
      <div v-else-if="error" role="alert" class="rounded-lg bg-rose-50 p-4 dark:bg-rose-950/30">
        <p>{{ t('usageContent.loadError') }}</p><button type="button" class="btn btn-secondary mt-3" @click="load">{{ t('usageContent.retry') }}</button>
      </div>
      <div v-else-if="!content" class="rounded-lg bg-gray-50 p-4 dark:bg-dark-800" data-testid="content-empty">
        <p>{{ t(emptyReason === 'expired' ? 'usageContent.expired' : emptyReason === 'capacity' ? 'usageContent.capacity' : 'usageContent.empty') }}</p>
        <button v-if="emptyReason !== 'expired'" type="button" class="btn btn-secondary mt-3" @click="load">{{ t('usageContent.retry') }}</button>
      </div>
      <template v-if="content && !loading">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="inline-flex gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-800" role="group" :aria-label="t('usageContent.viewMode')">
            <button v-for="view in views" :key="view.value" type="button" class="rounded-md px-3 py-1.5 text-sm"
              :class="mode === view.value ? 'bg-white font-medium shadow-sm dark:bg-dark-600' : 'text-gray-500'"
              :aria-pressed="mode === view.value" @click="mode = view.value">{{ t(view.label) }}</button>
          </div>
          <span class="text-xs text-gray-500">{{ t('usageContent.expiresAt', { time: formatDateTime(content.expires_at) }) }}</span>
        </div>
        <label v-if="captures.length > 1" class="block text-sm">{{ t('usageContent.captureSelection') }}
          <select v-model="selected" class="input mt-1 w-full" @change="load">
            <option v-for="capture in captures" :key="capture.capture_id" :value="capture.capture_id">{{ formatDateTime(capture.created_at) }} · {{ capture.protocol }} · {{ capture.status }}</option>
          </select>
        </label>
        <p v-if="content.partial" role="alert" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-300">{{ t('usageContent.partial') }}</p>
        <div class="grid gap-4 lg:grid-cols-2">
          <section class="min-w-0 rounded-xl border border-gray-200 p-4 dark:border-dark-600" data-testid="content-input">
            <header class="mb-3 flex items-center justify-between gap-2"><h3 class="font-semibold">{{ t('usageContent.input') }}</h3>
              <button type="button" class="text-xs text-primary-600 dark:text-primary-400" @click="copy(content.request_body)">{{ t('usageContent.copy') }}</button>
            </header>
            <p v-if="content.request_truncated" class="mb-3 text-xs text-amber-700 dark:text-amber-300">{{ t('usageContent.truncated', { stored: bytes(content.request_stored_bytes), total: bytes(content.request_bytes) }) }}</p>
            <p v-if="content.request_omitted === 'binary_content'" class="text-sm text-gray-500">{{ t('usageContent.binaryOmitted') }}</p>
            <div v-else-if="mode === 'conversation' && messages.length" class="max-h-[65vh] space-y-3 overflow-auto">
              <article v-for="(message, index) in messages" :key="index" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                <div class="mb-2 text-xs font-semibold text-primary-600 dark:text-primary-400">{{ message.role }}</div>
                <pre class="whitespace-pre-wrap break-words font-sans text-sm">{{ message.text }}</pre>
              </article>
            </div>
            <textarea v-else readonly spellcheck="false" rows="16" class="w-full resize-y rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
              :aria-label="t('usageContent.input')" :value="rawRequest" />
          </section>
          <section class="min-w-0 rounded-xl border border-gray-200 p-4 dark:border-dark-600" data-testid="content-output">
            <header class="mb-3 flex items-center justify-between gap-2"><h3 class="font-semibold">{{ t('usageContent.output') }}</h3>
              <button type="button" class="text-xs text-primary-600 dark:text-primary-400" @click="copy(content.response_body)">{{ t('usageContent.copy') }}</button>
            </header>
            <p v-if="content.response_truncated" class="mb-3 text-xs text-amber-700 dark:text-amber-300">{{ t('usageContent.truncated', { stored: bytes(content.response_stored_bytes), total: bytes(content.response_bytes) }) }}</p>
            <p v-if="content.response_omitted === 'binary_content'" class="text-sm text-gray-500">{{ t('usageContent.binaryOmitted') }}</p>
            <div v-else-if="mode === 'conversation' && hasReadableOutput" class="max-h-[65vh] space-y-4 overflow-auto">
              <div v-if="output.reasoning" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800"><h4 class="mb-2 text-xs font-semibold text-gray-500">{{ t('usageContent.reasoning') }}</h4><pre class="whitespace-pre-wrap break-words font-sans text-sm">{{ output.reasoning }}</pre></div>
              <pre v-if="output.text" class="whitespace-pre-wrap break-words font-sans text-sm">{{ output.text }}</pre>
              <div v-for="(tool, index) in output.tools" :key="index" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800"><h4 class="mb-2 text-xs font-semibold text-gray-500">{{ t('usageContent.tools') }}</h4><pre class="whitespace-pre-wrap break-words font-mono text-xs">{{ tool }}</pre></div>
            </div>
            <textarea v-else readonly spellcheck="false" rows="16" class="w-full resize-y rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
              :aria-label="t('usageContent.output')" :value="rawResponse" />
          </section>
        </div>
        <p v-if="copied" role="status" class="text-xs text-emerald-600">{{ t('usageContent.copied') }}</p>
        <p v-if="copyFailed" role="alert" class="text-xs text-rose-600">{{ t('usageContent.copyFailed') }}</p>
        <p class="break-all font-mono text-[10px] text-gray-400">{{ content.client_request_id }} · {{ content.protocol }} · {{ content.path }}</p>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWindowSize } from '@vueuse/core'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { formatDateTime } from '@/utils/format'
import { formatUsageRawBody, usageRequestMessages, usageResponseOutput } from '@/utils/usageContent'
import { getUsageContent, type UsageContentDetail, type UsageContentRecord, type UsageContentReference } from '@/api/admin/usageContent'

const props = withDefaults(defineProps<{ record: UsageContentRecord | null; errorRecord?: boolean }>(), { errorRecord: false })
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const { width: viewportWidth } = useWindowSize()
const content = ref<UsageContentDetail | null>(null)
const captures = ref<UsageContentReference[]>([])
const selected = ref(''), loading = ref(false), error = ref(false), emptyReason = ref('not_recorded')
const copied = ref(false), copyFailed = ref(false), mode = ref<'conversation' | 'raw'>('conversation')
const views = [{ value: 'conversation', label: 'usageContent.conversation' }, { value: 'raw', label: 'usageContent.raw' }] as const
let controller: AbortController | undefined
let expiryTimer: ReturnType<typeof setTimeout> | undefined
const messages = computed(() => usageRequestMessages(content.value?.request_body ?? ''))
const output = computed(() => usageResponseOutput(content.value?.response_body ?? ''))
const hasReadableOutput = computed(() => !!output.value.text || !!output.value.reasoning || output.value.tools.length > 0)
const rawRequest = computed(() => formatUsageRawBody(content.value?.request_body ?? ''))
const rawResponse = computed(() => formatUsageRawBody(content.value?.response_body ?? ''))
const bytes = (value: number) => value < 1024 ? `${value} B` : value < 1048576 ? `${(value / 1024).toFixed(1)} KiB` : `${(value / 1048576).toFixed(2)} MiB`

function reset() {
  controller?.abort()
  controller = undefined
  clearTimeout(expiryTimer)
  expiryTimer = undefined
  content.value = null
  captures.value = []
  copied.value = false
  copyFailed.value = false
  loading.value = false
}
function expire() {
  reset()
  emptyReason.value = 'expired'
}
function close() { reset(); emit('close') }
function checkExpiry() {
  if (content.value && Date.now() >= Date.parse(content.value.expires_at)) expire()
}

async function load() {
  if (!props.record) return
  controller?.abort()
  clearTimeout(expiryTimer)
  content.value = null
  copied.value = false
  copyFailed.value = false
  const current = new AbortController(), recordID = props.record.id
  controller = current
  loading.value = true
  error.value = false
  try {
    const result = await getUsageContent(recordID, { signal: current.signal, captureId: selected.value || undefined, errorRecord: props.errorRecord })
    if (controller !== current || current.signal.aborted) return
    captures.value = result.captures
    content.value = result.content
    selected.value = result.content?.capture_id ?? ''
    emptyReason.value = result.empty_reason
    if (content.value) {
      const remaining = Date.parse(content.value.expires_at) - Date.now()
      if (remaining <= 0 || !Number.isFinite(remaining)) expire()
      else expiryTimer = setTimeout(expire, remaining)
    }
  } catch {
    if (controller === current && !current.signal.aborted) error.value = true
  } finally {
    if (controller === current) loading.value = false
  }
}

async function copy(value: string) {
  try { await navigator.clipboard.writeText(value); copied.value = true; copyFailed.value = false }
  catch { copyFailed.value = true }
}

watch(() => [props.record?.id, props.errorRecord], () => {
  reset()
  selected.value = ''
  error.value = false
  emptyReason.value = 'not_recorded'
  mode.value = 'conversation'
  if (props.record) void load()
}, { immediate: true })
document.addEventListener('visibilitychange', checkExpiry)
onUnmounted(() => { reset(); document.removeEventListener('visibilitychange', checkExpiry) })
</script>
