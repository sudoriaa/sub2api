type ObjectValue = Record<string, unknown>
const object = (value: unknown): ObjectValue => value !== null && typeof value === 'object' && !Array.isArray(value) ? value as ObjectValue : {}
const list = (value: unknown): unknown[] => Array.isArray(value) ? value : []
const str = (value: unknown): string => typeof value === 'string' ? value : ''
const json = (value: unknown): string => JSON.stringify(value, null, 2) ?? ''

function parse(raw: string): unknown {
  try { return JSON.parse(raw) } catch { return null }
}

export function formatUsageRawBody(raw: string): string {
  const parsed = parse(raw)
  return parsed === null ? raw : json(parsed)
}

export interface ConversationMessage { role: string; text: string }
export interface ConversationOutput { text: string; reasoning: string; tools: string[] }

function contentText(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) return value.map(contentText).filter(Boolean).join('\n')
  const v = object(value)
  if (typeof v.text === 'string') return v.text
  if (typeof v.thinking === 'string') return v.thinking
  if (typeof v.output === 'string') return v.output
  if (v.content != null) return contentText(v.content)
  if (v.parts != null) return contentText(v.parts)
  // Do not render external images/URLs or inline media in a sensitive log view.
  if (['image', 'image_url', 'input_image', 'input_audio'].includes(str(v.type)) || v.inlineData || v.fileData) return `[${str(v.type) || 'media'}]`
  return Object.keys(v).length ? json(v) : ''
}

export function usageRequestMessages(raw: string): ConversationMessage[] {
  const request = object(parse(raw))
  const messages: ConversationMessage[] = []
  if (request.instructions != null) messages.push({ role: 'system', text: contentText(request.instructions) })
  if (request.system != null) messages.push({ role: 'system', text: contentText(request.system) })
  const entries = request.messages ?? request.input ?? request.contents
  if (typeof entries === 'string') messages.push({ role: 'user', text: entries })
  else for (const entry of list(entries)) {
    if (typeof entry === 'string') { messages.push({ role: 'user', text: entry }); continue }
    const item = object(entry)
    const role = str(item.role) || (str(item.type) === 'function_call_output' ? 'tool' : str(item.type) === 'function_call' ? 'assistant' : 'user')
    let text = contentText(item.content ?? item.parts ?? item)
    if (item.tool_calls) text += `\n${json(item.tool_calls)}`
    messages.push({ role, text })
  }
  if (!messages.length && request.prompt != null) messages.push({ role: 'user', text: contentText(request.prompt) })
  return messages
}

function outputObject(value: unknown): ConversationOutput {
  const v = object(value)
  const result: ConversationOutput = { text: '', reasoning: '', tools: [] }
  const text: string[] = [], reasoning: string[] = []
  for (const choice of list(v.choices)) {
    const c = object(choice), message = object(c.message)
    text.push(contentText(message.content ?? c.text))
    reasoning.push(str(message.reasoning_content) || str(message.reasoning))
    result.tools.push(...list(message.tool_calls).map(json))
    if (message.function_call) result.tools.push(json(message.function_call))
  }
  for (const candidate of list(v.candidates)) text.push(contentText(object(candidate).content))
  for (const block of list(v.output ?? v.content)) {
    const b = object(block)
    if (str(b.type) === 'reasoning') reasoning.push(contentText(b.summary))
    else if (str(b.type) === 'thinking') reasoning.push(str(b.thinking))
    else if (['function_call', 'tool_use'].includes(str(b.type))) result.tools.push(json(b))
    else text.push(contentText(b))
  }
  if (!text.filter(Boolean).length) text.push(str(v.output_text) || str(v.text) || (typeof v.content === 'string' ? v.content : ''))
  result.text = text.filter(Boolean).join('\n')
  result.reasoning = reasoning.filter(Boolean).join('\n')
  return result
}

export function usageResponseOutput(raw: string): ConversationOutput {
  const parsed = parse(raw)
  if (parsed !== null) return outputObject(parsed)
  const result: ConversationOutput = { text: '', reasoning: '', tools: [] }
  const toolDeltas = new Map<string, { name: string; arguments: string }>()
  let final: ConversationOutput | undefined
  // SSE events can contain multiple data lines and CRLF. A truncated last
  // event is ignored in the readable view; the raw view retains every byte.
  for (const event of raw.split(/\r?\n\r?\n/)) {
    const data = event.split(/\r?\n/).filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart()).join('\n')
    if (!data || data === '[DONE]') continue
    const value = parse(data)
    if (value === null) continue
    const v = object(value), type = str(v.type), delta = object(v.delta)
    if (['response.completed', 'response.done'].includes(type)) final = outputObject(v.response)
    if (type === 'response.output_text.delta') result.text += str(v.delta)
    if (type === 'response.reasoning_summary_text.delta' || type === 'response.reasoning_text.delta') result.reasoning += str(v.delta)
    if (type === 'response.output_text.done' && !result.text) result.text = str(v.text)
    if (type === 'content_block_delta') {
      result.text += str(delta.text)
      result.reasoning += str(delta.thinking)
      if (delta.partial_json != null) {
        const key = `anthropic-${v.index}`
        const tool = toolDeltas.get(key) ?? { name: '', arguments: '' }
        tool.arguments += str(delta.partial_json)
        toolDeltas.set(key, tool)
      }
    }
    if (type === 'content_block_start') {
      const block = object(v.content_block)
      if (str(block.type) === 'tool_use') toolDeltas.set(`anthropic-${v.index}`, { name: str(block.name), arguments: '' })
      else if (str(block.type) === 'thinking') result.reasoning += str(block.thinking)
      else result.text += str(block.text)
    }
    if (type === 'response.output_item.done') {
      const item = object(v.item)
      if (str(item.type) === 'function_call') result.tools.push(json(item))
    }
    for (const choice of list(v.choices)) {
      const c = object(choice), d = object(c.delta)
      result.text += str(d.content)
      result.reasoning += str(d.reasoning_content) || str(d.reasoning)
      for (const toolValue of list(d.tool_calls)) {
        const t = object(toolValue), f = object(t.function), key = `chat-${c.index}-${t.index}`
        const tool = toolDeltas.get(key) ?? { name: '', arguments: '' }
        tool.name += str(f.name)
        tool.arguments += str(f.arguments)
        toolDeltas.set(key, tool)
      }
    }
    for (const candidate of list(v.candidates)) result.text += contentText(object(candidate).content)
  }
  for (const tool of toolDeltas.values()) result.tools.push(`${tool.name}\n${tool.arguments}`.trim())
  // Completed Responses events often repeat the full text already delivered
  // in deltas. Prefer the completed representation rather than displaying twice.
  if (final) {
    if (final.text) result.text = final.text
    if (final.reasoning) result.reasoning = final.reasoning
    if (final.tools.length) result.tools = final.tools
  }
  return result
}
