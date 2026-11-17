package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// The threshold tests historically shared this helper with forecast tests.
// Upstream removed the forecast feature; keep the independent settings reader
// so that the custom threshold regression coverage remains intact.
func readForecastSettings(t *testing.T, h *SettingHandler) gjson.Result {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return gjson.GetBytes(rec.Body.Bytes(), "data")
}
