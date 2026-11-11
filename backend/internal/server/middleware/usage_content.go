package middleware

import (
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
	"github.com/gin-gonic/gin"
)

// UsageContent runs after authentication, before admission/model rewrites. It
// tees existing reads/writes; it never prereads, buffers or delays a response.
func UsageContent(manager *usagecontent.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !manager.Enabled() {
			c.Next()
			return
		}
		key, ok := GetAPIKeyFromContext(c)
		if !ok || key == nil {
			c.Next()
			return
		}
		clientID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			connection := usagecontent.NewConnection(manager, key.ID, clientID, c.Request.URL.Path)
			c.Request = c.Request.WithContext(usagecontent.WithConnection(c.Request.Context(), connection))
			defer connection.Close()
			c.Next()
			return
		}
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		s := manager.Begin(key.ID, clientID, c.Request.Method, c.Request.URL.Path, "http", c.GetHeader("Content-Type"))
		if s == nil {
			c.Next()
			return
		}
		c.Request = c.Request.WithContext(usagecontent.WithSession(c.Request.Context(), s))
		// Preserve the preread zero-copy path used by model/auth admission.
		if p, ok := c.Request.Body.(*pkghttputil.PrereadBody); ok {
			s.Input(p.Bytes(), c.GetHeader("Content-Type"))
		} else if c.Request.Body != nil && (c.GetHeader("Content-Encoding") == "" || strings.EqualFold(c.GetHeader("Content-Encoding"), "identity")) {
			c.Request.Body = &usageContentBody{ReadCloser: c.Request.Body, session: s}
		}
		writer := &usageContentWriter{ResponseWriter: c.Writer, session: s}
		c.Writer = writer
		defer func() { s.Finish(writer.Status(), writer.failed || c.Request.Context().Err() != nil) }()
		c.Next()
	}
}

type usageContentBody struct {
	io.ReadCloser
	session *usagecontent.Session
}

func (b *usageContentBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.session.InputBytes(p[:n])
	}
	return n, err
}

type usageContentWriter struct {
	gin.ResponseWriter
	session *usagecontent.Session
	failed  bool
}

func (w *usageContentWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if n > 0 {
		w.session.Output(p[:n], w.Header().Get("Content-Type"))
	}
	w.failed = w.failed || err != nil
	return n, err
}

func (w *usageContentWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *usageContentWriter) Flush() { _ = w.FlushError() }
func (w *usageContentWriter) FlushError() error {
	err := service.FlushGatewayResponse(w.ResponseWriter)
	w.failed = w.failed || err != nil
	return err
}
func (w *usageContentWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

var _ http.Flusher = (*usageContentWriter)(nil)
