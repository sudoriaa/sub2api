import { formatUsageRawBody, usageRequestMessages, usageResponseOutput, type ConversationMessage, type ConversationOutput } from './usageContent'

export interface UsageContentPresentation {
  messages: ConversationMessage[]
  output: ConversationOutput
  rawRequest: string
  rawResponse: string
}

export function emptyUsageContentPresentation(): UsageContentPresentation {
  return { messages: [], output: { text: '', reasoning: '', tools: [] }, rawRequest: '', rawResponse: '' }
}

// The worker and small-payload synchronous path share exactly one parser.
// Raw formatting is never performed while only the readable view is needed.
export function parseUsageContentPresentation(request: string, response: string, mode: 'raw' | 'conversation'): UsageContentPresentation {
  const result = emptyUsageContentPresentation()
  if (mode === 'raw') {
    result.rawRequest = formatUsageRawBody(request)
    result.rawResponse = formatUsageRawBody(response)
  } else {
    result.messages = usageRequestMessages(request)
    result.output = usageResponseOutput(response)
    if (!result.messages.length) result.rawRequest = request
    if (!result.output.text && !result.output.reasoning && !result.output.tools.length) result.rawResponse = response
  }
  return result
}
