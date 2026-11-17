import { describe, expect, it } from 'vitest'
import { parseUsageContentPresentation } from '../usageContentPresentation'

describe('Usage content presentation', () => {
  it('only prepares the selected mode and safely falls back for truncated JSON', () => {
    const request = '{"input":"private"}'
    const response = 'data: {"type":"response.output_text.delta","delta":"answer"}\n\n'
    const readable = parseUsageContentPresentation(request, response, 'conversation')
    expect(readable.messages[0].text).toBe('private')
    expect(readable.output.text).toBe('answer')
    expect(readable.rawRequest).toBe('')
    const raw = parseUsageContentPresentation(request, response, 'raw')
    expect(raw.messages).toEqual([])
    expect(raw.rawRequest).toContain('\n')
    expect(raw.rawResponse).toBe(response)
    expect(parseUsageContentPresentation('{"inpu', '', 'conversation').rawRequest).toBe('{"inpu')
  })
})
