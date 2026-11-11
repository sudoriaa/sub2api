package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ProvideUsageHandlerWithContent(usage *service.UsageService, keys *service.APIKeyService, admin service.AdminService,
	cleanup *service.UsageCleanupService, contents *usagecontent.Manager, ops *service.OpsService) *UsageHandler {
	h := NewUsageHandler(usage, keys, admin, cleanup)
	h.Contents, h.contentOps = contents, ops
	return h
}

// Content is intentionally not part of the user/observer usage API or DTO.
func (h *UsageHandler) Content(c *gin.Context) {
	if !allowUsageContent(c) {
		return
	}
	id, ok := contentRecordID(c)
	if !ok {
		return
	}
	log, err := h.usageService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.respondContent(c, usagecontent.Query{APIKeyID: log.APIKeyID, RequestID: log.RequestID}, log.CreatedAt)
}

// ErrorContent also handles unbilled upstream failures after authentication.
// The error log's trusted internal client ID + key ID identify its capture.
func (h *UsageHandler) ErrorContent(c *gin.Context) {
	if !allowUsageContent(c) {
		return
	}
	id, ok := contentRecordID(c)
	if !ok {
		return
	}
	if h.contentOps == nil {
		response.Error(c, http.StatusServiceUnavailable, "Error records are unavailable")
		return
	}
	log, err := h.contentOps.GetErrorLogByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	q := usagecontent.Query{ClientRequestID: log.ClientRequestID}
	if log.APIKeyID != nil {
		q.APIKeyID = *log.APIKeyID
	}
	h.respondContent(c, q, log.CreatedAt)
}

func allowUsageContent(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	role, _ := middleware.GetUserRoleFromContext(c)
	_, observer := service.ObserverGroupIDs(c.Request.Context())
	if role != service.RoleAdmin || observer || adminaccess.FromContext(c.Request.Context()) != nil {
		response.Forbidden(c, "Only an unrestricted administrator can view request content")
		return false
	}
	return true
}

func contentRecordID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid record ID")
		return 0, false
	}
	if capture := c.Query("capture_id"); capture != "" {
		if _, err := uuid.Parse(capture); err != nil {
			response.BadRequest(c, "Invalid capture ID")
			return 0, false
		}
	}
	return id, true
}

func (h *UsageHandler) respondContent(c *gin.Context, query usagecontent.Query, createdAt time.Time) {
	query.CaptureID = c.Query("capture_id")
	refs := []usagecontent.Reference{}
	var detail *usagecontent.Detail
	reason := "not_recorded"
	if !time.Now().Before(createdAt.Add(usagecontent.Retention)) {
		reason = "expired"
	}
	if h.Contents != nil && query.APIKeyID > 0 && (query.RequestID != "" || query.ClientRequestID != "") {
		readCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		var err error
		refs, detail, err = h.Contents.Read(readCtx, query)
		if err != nil && !errors.Is(err, usagecontent.ErrNotFound) {
			if errors.Is(err, usagecontent.ErrBusy) {
				c.Header("Retry-After", "1")
				response.Error(c, http.StatusTooManyRequests, "Request content reader is busy; try again")
			} else {
				response.Error(c, http.StatusInternalServerError, "Failed to load request content")
			}
			return
		}
	}
	if refs == nil {
		refs = []usagecontent.Reference{}
	}
	if detail != nil {
		reason = ""
	} else if reason != "expired" && h.Contents.Stats().StorageFull {
		reason = "capacity"
	}
	response.Success(c, gin.H{"content": detail, "captures": refs, "retention_hours": 24,
		"enabled": h.Contents.Enabled(), "empty_reason": reason, "stats": h.Contents.Stats()})
}
