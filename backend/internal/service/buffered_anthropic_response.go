package service

import (
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// bufferedAnthropicResponseAssembler reconstructs one non-streamed response.
// Block starts append in encounter order; deltas address that slice by index
// and may be interleaved. Keep builders at stable addresses (a strings.Builder
// must not be copied after its first write).
type bufferedAnthropicResponseAssembler struct {
	message           *apicompat.AnthropicResponse
	blocks            []*bufferedAnthropicBlock
	includeSignatures bool // Only Responses historically consumed signature_delta.
}

type bufferedAnthropicBlock struct {
	text, thinking, signature strings.Builder
	input                     json.RawMessage
	inputState                emptyJSONObjectState
}

func (a *bufferedAnthropicResponseAssembler) seed(block *apicompat.AnthropicContentBlock) {
	b := &bufferedAnthropicBlock{input: block.Input[:len(block.Input):len(block.Input)]}
	b.inputState = b.inputState.consume(b.input)
	b.text.WriteString(block.Text)
	b.thinking.WriteString(block.Thinking)
	if a.includeSignatures {
		b.signature.WriteString(block.Signature)
	}
	a.blocks = append(a.blocks, b)
}

func (a *bufferedAnthropicResponseAssembler) apply(event *apicompat.AnthropicStreamEvent, usage *ClaudeUsage) {
	switch event.Type {
	case "message_start":
		if event.Message != nil {
			a.message = event.Message
			a.blocks = make([]*bufferedAnthropicBlock, 0, len(a.message.Content))
			for i := range a.message.Content {
				a.seed(&a.message.Content[i])
			}
			mergeAnthropicUsage(usage, a.message.Usage)
		}
	case "message_delta":
		if event.Usage != nil {
			mergeAnthropicUsage(usage, *event.Usage)
		}
		if event.Delta != nil && event.Delta.StopReason != "" && a.message != nil {
			a.message.StopReason = apicompat.AnthropicStopReasonPtr(event.Delta.StopReason)
		}
	case "content_block_start":
		if a.message != nil && event.ContentBlock != nil {
			a.message.Content = append(a.message.Content, *event.ContentBlock)
			a.seed(event.ContentBlock)
		}
	case "content_block_delta":
		if event.Delta == nil || event.Index == nil || *event.Index < 0 || *event.Index >= len(a.blocks) {
			return
		}
		b := a.blocks[*event.Index]
		switch d := event.Delta; d.Type {
		case "text_delta":
			b.text.WriteString(d.Text)
		case "thinking_delta":
			b.thinking.WriteString(d.Thinking)
		case "signature_delta":
			if a.includeSignatures {
				b.signature.WriteString(d.Signature)
			}
		case "input_json_delta":
			b.appendInput(d.PartialJSON)
		}
	}
}

// response materializes even unfinished blocks when upstream ends without
// sending a terminal event. No block_stop or message_stop is required.
func (a *bufferedAnthropicResponseAssembler) response() *apicompat.AnthropicResponse {
	for i, b := range a.blocks {
		block := &a.message.Content[i]
		block.Text = b.text.String()
		block.Thinking = b.thinking.String()
		if a.includeSignatures {
			block.Signature = b.signature.String()
		}
		block.Input = b.input
	}
	return a.message
}

func (b *bufferedAnthropicBlock) appendInput(fragment string) {
	// Test for a complete {} *before every fragment*, not just at block start:
	// a placeholder can itself arrive in pieces. Carry the recognition state
	// across fragments so even a long run of leading whitespace is scanned once.
	if len(b.input) == 0 || b.inputState == emptyObjectComplete {
		b.input = json.RawMessage(fragment)
		b.inputState = emptyObjectStart.consume(b.input)
		return
	}
	start := len(b.input)
	b.input = append(b.input, fragment...)
	b.inputState = b.inputState.consume(b.input[start:])
}

type emptyJSONObjectState uint8

const (
	emptyObjectStart emptyJSONObjectState = iota
	emptyObjectOpen
	emptyObjectComplete
	notEmptyObject
)

func (s emptyJSONObjectState) consume(b []byte) emptyJSONObjectState {
	if s == notEmptyObject {
		return s
	}
	for _, c := range b {
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		case c == '{' && s == emptyObjectStart:
			s = emptyObjectOpen
		case c == '}' && s == emptyObjectOpen:
			s = emptyObjectComplete
		default:
			return notEmptyObject
		}
	}
	return s
}

func isEmptyJSONObjectBytes(b []byte) bool {
	return emptyObjectStart.consume(b) == emptyObjectComplete
}
