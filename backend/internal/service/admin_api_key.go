package service

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
)

// 2026-09-13 coder(lq): AdminUpdateAPIKey updates an API key without applying the owner's group restrictions.
func (s *adminServiceImpl) AdminUpdateAPIKey(ctx context.Context, keyID int64, req UpdateAPIKeyRequest) (*APIKey, error) {
	if err := validateUpdateAPIKeyRequest(req); err != nil {
		return nil, err
	}

	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}

	// 2026-09-13 coder(lq): Keep group changes on the existing admin path because it also handles
	// subscription checks and exclusive-group permission updates.
	if req.GroupID != nil {
		groupResult, err := s.AdminUpdateAPIKeyGroupID(ctx, keyID, req.GroupID)
		if err != nil {
			return nil, err
		}
		apiKey = groupResult.APIKey
		req.GroupID = nil
		if req.Name == nil && req.Status == nil && req.IPWhitelist == nil && req.IPBlacklist == nil &&
			req.Quota == nil && req.ExpiresAt == nil && !req.ClearExpiration && req.ResetQuota == nil &&
			req.RateLimit5h == nil && req.RateLimit1d == nil && req.RateLimit7d == nil &&
			(req.ResetRateLimitUsage == nil || !*req.ResetRateLimitUsage) {
			return apiKey, nil
		}
		// 2026-09-13 coder(lq): Reload after the group update so the remaining fields are applied to
		// the current row rather than the pre-update snapshot.
		apiKey, err = s.apiKeyRepo.GetByID(ctx, keyID)
		if err != nil {
			return nil, fmt.Errorf("reload api key: %w", err)
		}
	}

	if req.IPWhitelist != nil {
		if invalid := ip.ValidateIPPatterns(*req.IPWhitelist); len(invalid) > 0 {
			return nil, fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}
	if req.IPBlacklist != nil {
		if invalid := ip.ValidateIPPatterns(*req.IPBlacklist); len(invalid) > 0 {
			return nil, fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}

	var fields APIKeyUpdateFields
	originalStatus := apiKey.Status

	if req.Name != nil {
		apiKey.Name = html.EscapeString(*req.Name)
		fields.Name = true
	}
	if req.Status != nil {
		apiKey.Status = *req.Status
		fields.Status = true
	}
	if req.Quota != nil {
		apiKey.Quota = *req.Quota
		fields.Quota = true
		if apiKey.Status == StatusAPIKeyQuotaExhausted && (*req.Quota <= 0 || *req.Quota > apiKey.QuotaUsed) {
			apiKey.Status = StatusActive
		}
	}
	if req.ResetQuota != nil && *req.ResetQuota {
		apiKey.QuotaUsed = 0
		fields.QuotaUsed = true
		if apiKey.Status == StatusAPIKeyQuotaExhausted {
			apiKey.Status = StatusActive
		}
	}
	if req.ClearExpiration {
		apiKey.ExpiresAt = nil
		fields.ExpiresAt = true
	} else if req.ExpiresAt != nil {
		apiKey.ExpiresAt = req.ExpiresAt
		fields.ExpiresAt = true
		if apiKey.Status == StatusAPIKeyExpired && time.Now().Before(*req.ExpiresAt) {
			apiKey.Status = StatusActive
		}
	}
	if req.IPWhitelist != nil {
		apiKey.IPWhitelist = *req.IPWhitelist
		fields.IPRules = true
	}
	if req.IPBlacklist != nil {
		apiKey.IPBlacklist = *req.IPBlacklist
		fields.IPRules = true
	}
	if req.RateLimit5h != nil {
		apiKey.RateLimit5h = *req.RateLimit5h
		fields.RateLimits = true
	}
	if req.RateLimit1d != nil {
		apiKey.RateLimit1d = *req.RateLimit1d
		fields.RateLimits = true
	}
	if req.RateLimit7d != nil {
		apiKey.RateLimit7d = *req.RateLimit7d
		fields.RateLimits = true
	}
	resetRateLimit := req.ResetRateLimitUsage != nil && *req.ResetRateLimitUsage
	if resetRateLimit {
		apiKey.Usage5h = 0
		apiKey.Usage1d = 0
		apiKey.Usage7d = 0
		apiKey.Window5hStart = nil
		apiKey.Window1dStart = nil
		apiKey.Window7dStart = nil
		fields.RateLimitUsage = true
	}
	if apiKey.Status != originalStatus {
		fields.Status = true
	}

	if fields.IsEmpty() {
		return apiKey, nil
	}
	if err := s.apiKeyRepo.Update(ctx, apiKey, fields); err != nil {
		return nil, fmt.Errorf("update api key: %w", err)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}
	if resetRateLimit && s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateAPIKeyRateLimit(ctx, apiKey.ID)
	}
	return apiKey, nil
}

// 2026-09-13 coder(lq): AdminDeleteAPIKey removes an API key with administrator privileges.
func (s *adminServiceImpl) AdminDeleteAPIKey(ctx context.Context, keyID int64) error {
	key, _, err := s.apiKeyRepo.GetKeyAndOwnerID(ctx, keyID)
	if err != nil {
		return fmt.Errorf("get api key: %w", err)
	}
	if err := s.apiKeyRepo.DeleteWithAudit(ctx, keyID); err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, key)
	}
	return nil
}
