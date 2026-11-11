import { describe, expect, it } from 'vitest'
import { formatUsageRawBody, usageRequestMessages, usageResponseOutput } from '../usageContent'

const sse = (...events: unknown[]) => events.map(event => `data: ${JSON.stringify(event)}\n\n`).join('')

describe('Request conversation presentation', () => {
  it('keeps system prompts, history, tool calls and tool results', () => {
    const messages = usageRequestMessages(JSON.stringify({ instructions: 'system rules', input: [
      { role: 'user', content: [{ type: 'input_text', text: 'first question' }] },
      { role: 'assistant', content: 'first answer' },
      { type: 'function_call', name: 'lookup', arguments: '{"id":1}' },
      { type: 'function_call_output', call_id: 'call-1', output: 'tool result' }
    ] }))
    expect(messages.map(item => item.role)).toEqual(['system', 'user', 'assistant', 'assistant', 'tool'])
    expect(messages.map(item => item.text).join('\n')).toContain('system rules')
    expect(messages[1].text).toBe('first question')
    expect(messages[3].text).toContain('lookup')
    expect(messages[4].text).toBe('tool result')
  })
  it('supports Anthropic and Gemini request shapes', () => {
    expect(usageRequestMessages('{"system":[{"type":"text","text":"rules"}],"messages":[{"role":"user","content":"hello"}]}')).toEqual([{ role: 'system', text: 'rules' }, { role: 'user', text: 'hello' }])
    expect(usageRequestMessages('{"contents":[{"role":"user","parts":[{"text":"Gemini question"}]}]}')[0].text).toBe('Gemini question')
  })
  it('does not load media URLs and keeps malformed raw data available', () => {
    expect(usageRequestMessages('{"input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.test/private.png"}]}]}')[0].text).toBe('[input_image]')
    const truncated = '{"input":"unfinished'
    expect(usageRequestMessages(truncated)).toEqual([])
    expect(formatUsageRawBody(truncated)).toBe(truncated)
  })
})

describe('Response conversation presentation', () => {
  it('reassembles chat SSE text, returned thinking and tool argument deltas', () => {
    const raw = sse(
      { choices: [{ index: 0, delta: { reasoning_content: 'thinking ' } }] },
      { choices: [{ index: 0, delta: { content: '你好', tool_calls: [{ index: 0, function: { name: 'lookup', arguments: '{"id":' } }] } }] },
      { choices: [{ index: 0, delta: { content: '世界', tool_calls: [{ index: 0, function: { arguments: '1}' } }] } }] }
    ) + 'data: [DONE]\n\n'
    expect(usageResponseOutput(raw)).toEqual({ text: '你好世界', reasoning: 'thinking ', tools: ['lookup\n{"id":1}'] })
  })
  it('does not duplicate completed Responses text or tool calls', () => {
    const raw = sse(
      { type: 'response.output_text.delta', delta: 'hello ' },
      { type: 'response.output_text.delta', delta: 'world' },
      { type: 'response.reasoning_summary_text.delta', delta: 'returned summary' },
      { type: 'response.completed', response: { output: [{ type: 'message', content: [{ type: 'output_text', text: 'hello world' }] }] } }
    )
    expect(usageResponseOutput(raw)).toEqual({ text: 'hello world', reasoning: 'returned summary', tools: [] })
  })
  it('handles Anthropic thinking, text and tool-use input', () => {
    const raw = sse(
      { type: 'content_block_start', index: 0, content_block: { type: 'thinking', thinking: '' } },
      { type: 'content_block_delta', index: 0, delta: { type: 'thinking_delta', thinking: 'reason' } },
      { type: 'content_block_delta', index: 1, delta: { type: 'text_delta', text: 'answer' } },
      { type: 'content_block_start', index: 2, content_block: { type: 'tool_use', name: 'tool', input: {} } },
      { type: 'content_block_delta', index: 2, delta: { type: 'input_json_delta', partial_json: '{"q":"x"}' } }
    )
    expect(usageResponseOutput(raw)).toEqual({ text: 'answer', reasoning: 'reason', tools: ['tool\n{"q":"x"}'] })
  })
  it('supports nonstream Responses, chat, Anthropic and Gemini', () => {
    expect(usageResponseOutput('{"choices":[{"message":{"content":"chat"}}]}').text).toBe('chat')
    expect(usageResponseOutput('{"content":[{"type":"text","text":"anthropic"}]}').text).toBe('anthropic')
    expect(usageResponseOutput('{"candidates":[{"content":{"parts":[{"text":"gemini"}]}}]}').text).toBe('gemini')
    expect(usageResponseOutput('{"output":[{"type":"message","content":[{"type":"output_text","text":"responses"}]}]}').text).toBe('responses')
  })
  it('keeps raw SSE including truncated events and tolerates CRLF/multiline data', () => {
    const raw = 'data: {"choices":\r\ndata: [{"delta":{"content":"ok"}}]}\r\n\r\ndata: {"incomplete":'
    expect(usageResponseOutput(raw).text).toBe('ok')
    expect(formatUsageRawBody(raw)).toBe(raw)
  })
})
