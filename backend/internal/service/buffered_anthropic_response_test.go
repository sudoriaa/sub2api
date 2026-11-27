//go:build unit

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBufferedAnthropicResponseAssembler(t *testing.T) {
	for _, signatures := range []bool{false, true} {
		a := bufferedAnthropicResponseAssembler{includeSignatures: signatures}
		var u ClaudeUsage
		apply := func(e apicompat.AnthropicStreamEvent) { a.apply(&e, &u) }
		delta := func(i int, d apicompat.AnthropicDelta) {
			apply(apicompat.AnthropicStreamEvent{Type: "content_block_delta", Index: &i, Delta: &d})
		}
		text := apicompat.AnthropicDelta{Type: "text_delta", Text: "!"}
		delta(0, text)
		require.Nil(t, a.response())
		apply(apicompat.AnthropicStreamEvent{Type: "message_start", Message: &apicompat.AnthropicResponse{Content: []apicompat.AnthropicContentBlock{{Text: "discard"}}}})
		delta(0, text)
		msg := &apicompat.AnthropicResponse{Content: []apicompat.AnthropicContentBlock{{Type: "text", Text: "new"}}}
		apply(apicompat.AnthropicStreamEvent{Type: "message_start", Message: msg})
		apply(apicompat.AnthropicStreamEvent{Type: "content_block_start", ContentBlock: &apicompat.AnthropicContentBlock{Type: "thinking", Thinking: "think", Signature: "sig"}})
		apply(apicompat.AnthropicStreamEvent{Type: "content_block_start", ContentBlock: &apicompat.AnthropicContentBlock{Type: "tool_use", Input: json.RawMessage("{ }")}})
		apply(apicompat.AnthropicStreamEvent{Type: "message_start"}) // nil does not replace
		delta(2, apicompat.AnthropicDelta{Type: "input_json_delta", PartialJSON: " {"})
		delta(0, text)
		delta(1, apicompat.AnthropicDelta{Type: "signature_delta", Signature: "+"})
		delta(2, apicompat.AnthropicDelta{Type: "input_json_delta", PartialJSON: "\t}\n"})
		delta(1, apicompat.AnthropicDelta{Type: "thinking_delta", Thinking: "ing"})
		delta(2, apicompat.AnthropicDelta{Type: "input_json_delta", PartialJSON: "[1]"})
		delta(3, text)
		delta(-1, text)
		got := a.response()
		require.Same(t, msg, got)
		sig := "sig"
		if signatures {
			sig += "+"
		}
		require.Equal(t, []apicompat.AnthropicContentBlock{
			{Type: "text", Text: "new!"}, {Type: "thinking", Thinking: "thinking", Signature: sig}, {Type: "tool_use", Input: json.RawMessage("[1]")},
		}, got.Content)
	}
}

func TestBufferedAnthropicInputFragmentParity(t *testing.T) {
	// The previous implementation used json.Unmarshal to recognize {} at
	// every boundary. Compare both the standalone and incremental versions.
	for _, seed := range []string{"", "null", "{}", " \r{\t }\n", "{", "\u00a0{}", "[]", "{{}", "{\"x\":1}", "{}{}", "\f{}", "\xff"} {
		for _, fragments := range [][]string{{""}, {"", " {", "\r", "}", "\n", "{", " }", "true"}, {"\t", "}", "{", "}", "{\"x\":", "3}"}, {"\"k\":", "{}", "}"}} {
			var b bufferedAnthropicBlock
			b.input = json.RawMessage(seed)
			b.inputState = emptyObjectStart.consume(b.input)
			want := json.RawMessage(seed)
			for _, fragment := range fragments {
				var obj map[string]json.RawMessage
				empty := json.Unmarshal(want, &obj) == nil && obj != nil && len(obj) == 0
				require.Equal(t, empty, isEmptyJSONObjectBytes(want))
				gotStandalone := appendRawJSON(want, fragment)
				if empty {
					want = nil
				}
				want = json.RawMessage(string(want) + fragment)
				b.appendInput(fragment)
				require.Equal(t, want, gotStandalone)
				require.Equal(t, want, b.input)
			}
		}
	}
	require.Zero(t, testing.AllocsPerRun(100, func() {
		if !isEmptyJSONObjectBytes([]byte(" \t{\r\n}\t")) {
			panic("not empty")
		}
	}))
}

func TestBufferedAnthropicHandlersPartialEOF(t *testing.T) {
	stream := "event: message_start\ndata: " + `{"type":"message_start","message":{"content":[{"type":"text","text":"discarded"}]}}` +
		"\n\nevent: message_start\ndata: " + `{"type":"message_start","message":{"content":[{"type":"text","text":"partial "}]}}` +
		"\n\nevent: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"answer"}}` +
		"\n\nevent: content_block_stop\n" // no data line or message_stop
	for _, handler := range []string{"responses", "chat", "native-responses", "native-chat"} {
		t.Run(handler, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(stream))}
			now := time.Now()
			var err error
			switch handler {
			case "responses":
				_, err = (&GatewayService{}).handleResponsesBufferedStreamingResponse(resp, c, "model", "model", nil, now, apicompat.ResponsesClientToolMapping{})
			case "chat":
				_, err = (&GatewayService{}).handleCCBufferedFromAnthropic(resp, c, "model", "model", nil, now)
			case "native-responses":
				_, err = (&OpenAIGatewayService{}).handleResponsesBufferedFromNativeAnthropic(resp, c, "model", "model", "model", nil, now, apicompat.ResponsesClientToolMapping{})
			case "native-chat":
				_, err = (&OpenAIGatewayService{}).handleCCBufferedFromNativeAnthropic(resp, c, "model", "model", "model", nil, now)
			}
			require.NoError(t, err)
			require.Equal(t, 200, rec.Code)
			require.Contains(t, rec.Body.String(), "partial answer")
			require.NotContains(t, rec.Body.String(), "discarded")
		})
	}
}

func BenchmarkBufferedAnthropicAssembler(b *testing.B) {
	for _, n := range []int{128, 2048} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			idx := 0
			delta := apicompat.AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &apicompat.AnthropicDelta{Type: "text_delta", Text: "streamed text"}}
			// Incomplete placeholder until the last fragment: this is quadratic
			// if previous bytes are inspected again at each fragment boundary.
			input := apicompat.AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &apicompat.AnthropicDelta{Type: "input_json_delta", PartialJSON: "  "}}
			b.ReportAllocs()
			for b.Loop() {
				var a bufferedAnthropicResponseAssembler
				var u ClaudeUsage
				a.apply(&apicompat.AnthropicStreamEvent{Type: "message_start", Message: &apicompat.AnthropicResponse{Content: []apicompat.AnthropicContentBlock{{Input: json.RawMessage("{")}}}}, &u)
				for range n {
					a.apply(&delta, &u)
					a.apply(&input, &u)
				}
				block := a.response().Content[0]
				if len(block.Text) != n*len(delta.Delta.Text) || len(block.Input) != 1+2*n || !bytes.HasPrefix(block.Input, []byte("{")) {
					b.Fatal("lost deltas")
				}
			}
		})
	}
}
