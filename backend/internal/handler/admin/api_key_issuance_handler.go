package admin

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// 2026-09-13 coder(lq): Isolate admin-issued key delivery from the existing API key management handler.
type AdminAPIKeyIssuanceHandler struct {
	adminService  service.AdminService
	apiKeyCreator service.APIKeyCreator
}

// 2026-09-13 coder(lq): Keep the issuance constructor dependent on the narrow creator contract.
func NewAdminAPIKeyIssuanceHandler(adminService service.AdminService, apiKeyCreator service.APIKeyCreator) *AdminAPIKeyIssuanceHandler {
	return &AdminAPIKeyIssuanceHandler{
		adminService:  adminService,
		apiKeyCreator: apiKeyCreator,
	}
}

// 2026-09-13 coder(lq): AdminIssueAPIKeyRequest contains the administrator-controlled key settings.
type AdminIssueAPIKeyRequest struct {
	UserID        int64    `json:"user_id"`
	Name          string   `json:"name"`
	GroupID       *int64   `json:"group_id"`
	IPWhitelist   []string `json:"ip_whitelist"`
	IPBlacklist   []string `json:"ip_blacklist"`
	Quota         float64  `json:"quota"`
	ExpiresInDays *int     `json:"expires_in_days"`
	RateLimit5h   float64  `json:"rate_limit_5h"`
	RateLimit1d   float64  `json:"rate_limit_1d"`
	RateLimit7d   float64  `json:"rate_limit_7d"`
}

type adminIssueAPIKeyResponse struct {
	APIKey       *dto.APIKey `json:"api_key"`
	PlaintextKey string      `json:"plaintext_key"`
}

// 2026-09-13 coder(lq): Issue creates a key for an existing active user without requiring user login.
func (h *AdminAPIKeyIssuanceHandler) Issue(c *gin.Context) {
	var req AdminIssueAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.UserID <= 0 {
		response.BadRequest(c, "user_id must be greater than zero")
		return
	}
	if req.Name == "" {
		response.BadRequest(c, "name is required")
		return
	}
	if h.apiKeyCreator == nil {
		response.InternalError(c, "API key issuance is unavailable")
		return
	}

	executeAdminIdempotentJSON(c, "admin.api_keys.issue", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		user, err := h.adminService.GetUser(ctx, req.UserID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, service.ErrUserNotFound
		}
		if !user.IsActive() {
			return nil, service.ErrUserNotActive
		}

		apiKey, err := h.apiKeyCreator.Create(ctx, req.UserID, service.CreateAPIKeyRequest{
			Name:          req.Name,
			GroupID:       req.GroupID,
			IPWhitelist:   req.IPWhitelist,
			IPBlacklist:   req.IPBlacklist,
			Quota:         req.Quota,
			ExpiresInDays: req.ExpiresInDays,
			RateLimit5h:   req.RateLimit5h,
			RateLimit1d:   req.RateLimit1d,
			RateLimit7d:   req.RateLimit7d,
		})
		if err != nil {
			return nil, err
		}

		apiKeyDTO := dto.APIKeyFromService(apiKey)
		// 2026-09-13 coder(lq): Keep the one-time secret explicit and out of the reusable API key DTO.
		apiKeyDTO.Key = ""
		return adminIssueAPIKeyResponse{
			APIKey:       apiKeyDTO,
			PlaintextKey: apiKey.Key,
		}, nil
	})
}
