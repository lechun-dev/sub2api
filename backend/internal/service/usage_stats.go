package service

import (
	"context"
	"errors"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// 计费口径的 token 统计分组维度。
const (
	UsageStatsGroupNone   = "none"
	UsageStatsGroupUser   = "user"
	UsageStatsGroupDate   = "date"
	UsageStatsGroupModel  = "model"
	UsageStatsGroupGroup  = "group"
	UsageStatsGroupAPIKey = "api_key"
)

const (
	usageStatsDefaultLimit = 50
	usageStatsMaxLimit     = 200
)

// UsageStatsTotals 是 usage_logs 的汇总，口径与计费一致。
type UsageStatsTotals struct {
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
}

// UsageStatsAggregate 是按维度聚合后的一行。
type UsageStatsAggregate struct {
	Key       string `json:"key"`
	UserID    *int64 `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	Email     string `json:"email,omitempty"`
	GroupID   *int64 `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`
	APIKeyID  *int64 `json:"api_key_id,omitempty"`
	Model     string `json:"model,omitempty"`
	Date      string `json:"date,omitempty"`
	UsageStatsTotals
}

// UsageStatsAggregator 是 usage_logs 聚合查询的窄接口，由 usage_log 仓储实现。
type UsageStatsAggregator interface {
	AggregateUsageStats(ctx context.Context, filters usagestats.UsageLogFilters, groupBy string, limit int, descending bool) ([]*UsageStatsAggregate, *UsageStatsTotals, error)
}

// GetAggregatedStats 按用户/日期/模型等维度汇总 usage_logs 的 token 与费用。
func (s *UsageService) GetAggregatedStats(ctx context.Context, filters usagestats.UsageLogFilters, groupBy string, limit int, descending bool) ([]*UsageStatsAggregate, *UsageStatsTotals, error) {
	if s == nil || s.usageRepo == nil {
		return nil, nil, errors.New("usage statistics service is unavailable")
	}
	groupBy = strings.ToLower(strings.TrimSpace(groupBy))
	switch groupBy {
	case "", UsageStatsGroupNone:
		groupBy = UsageStatsGroupNone
	case UsageStatsGroupUser, UsageStatsGroupDate, UsageStatsGroupModel, UsageStatsGroupGroup, UsageStatsGroupAPIKey:
	default:
		return nil, nil, infraerrors.BadRequest("UNSUPPORTED_GROUP_BY", "unsupported group_by")
	}
	if limit <= 0 {
		limit = usageStatsDefaultLimit
	}
	if limit > usageStatsMaxLimit {
		limit = usageStatsMaxLimit
	}
	aggregator, ok := s.usageRepo.(UsageStatsAggregator)
	if !ok {
		return nil, nil, errors.New("usage statistics aggregation is unavailable")
	}
	return aggregator.AggregateUsageStats(ctx, filters, groupBy, limit, descending)
}
