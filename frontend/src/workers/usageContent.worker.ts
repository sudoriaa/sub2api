import { parseUsageContentPresentation } from '../utils/usageContentPresentation'

self.onmessage = (event: MessageEvent<{ request: string; response: string; mode: 'raw' | 'conversation' }>) => {
  const { request, response, mode } = event.data
  self.postMessage(parseUsageContentPresentation(request, response, mode))
}
