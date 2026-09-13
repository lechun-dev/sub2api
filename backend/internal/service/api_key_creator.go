package service

import "context"

// 2026-09-13 coder(lq): Keep admin key issuance dependent on a narrow creation contract for low-coupling upstream syncs.
type APIKeyCreator interface {
	Create(ctx context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error)
}
