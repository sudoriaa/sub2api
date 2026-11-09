package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIUpstreamAdmissionPauseResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, retryAfter := range []string{"300", "invalid", "300\r\nX-Injected: bad"} {
		t.Run(retryAfter, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			e := &service.UpstreamFailoverError{StatusCode: 429, Reason: service.OpenAIUpstreamAdmissionPausedReason, ResponseHeaders: http.Header{"Retry-After": {retryAfter}}, ResponseBody: []byte(`{"error":{"message":"untrusted-secret"}}`)}
			(&OpenAIGatewayHandler{}).handleFailoverExhausted(c, e, false)
			require.Equal(t, 429, rec.Code)
			require.Contains(t, rec.Body.String(), `"code":"upstream_admission_paused"`)
			require.NotContains(t, rec.Body.String(), "untrusted-secret")
			if retryAfter == "300" {
				require.Equal(t, "300", rec.Header().Get("Retry-After"))
			} else {
				require.Empty(t, rec.Header().Get("Retry-After"))
			}
		})
	}
}
