package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageStatsFakeRepo struct {
	service.UsageLogRepository
	items       []*service.UsageStatsAggregate
	totals      *service.UsageStatsTotals
	seenFilters usagestats.UsageLogFilters
	seenGroupBy string
	seenLimit   int
	seenDesc    bool
	calls       int
}

func (r *usageStatsFakeRepo) AggregateUsageStats(_ context.Context, filters usagestats.UsageLogFilters, groupBy string, limit int, descending bool) ([]*service.UsageStatsAggregate, *service.UsageStatsTotals, error) {
	r.calls++
	r.seenFilters = filters
	r.seenGroupBy = groupBy
	r.seenLimit = limit
	r.seenDesc = descending
	if r.totals == nil {
		r.totals = &service.UsageStatsTotals{}
	}
	return r.items, r.totals, nil
}

func newInternalUsageStatsHandler(repo *usageStatsFakeRepo) *UsageHandler {
	return NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
}

func TestUsageHandlerInternalStatsParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &usageStatsFakeRepo{
		items: []*service.UsageStatsAggregate{{
			Key: "7", Requests: 2, InputTokens: 10, OutputTokens: 20, TotalTokens: 30, TotalCost: 1.5,
		}},
		totals: &service.UsageStatsTotals{Requests: 2, InputTokens: 10, OutputTokens: 20, TotalTokens: 30, TotalCost: 1.5},
	}
	handler := newInternalUsageStatsHandler(repo)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet,
		"/api/v1/internal/usage/stats?user_id=7&group_by=user&limit=10&order=asc&start_time=2026-10-01&end_time=2026-10-31", nil)
	handler.InternalStats(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	data := decodeResponseData(t, recorder)
	require.Equal(t, "user", data["group_by"])
	require.Equal(t, "asc", data["order"])
	totals := data["totals"].(map[string]any)
	require.Equal(t, float64(30), totals["total_tokens"])
	items := data["items"].([]any)
	require.Len(t, items, 1)
	require.Equal(t, "7", items[0].(map[string]any)["key"])

	require.Equal(t, 1, repo.calls)
	require.Equal(t, int64(7), repo.seenFilters.UserID)
	require.Equal(t, "user", repo.seenGroupBy)
	require.Equal(t, 10, repo.seenLimit)
	require.False(t, repo.seenDesc)
	require.NotNil(t, repo.seenFilters.StartTime)
	require.Equal(t, "2026-10-01T00:00:00Z", repo.seenFilters.StartTime.Format(time.RFC3339))
	require.NotNil(t, repo.seenFilters.EndTime)
	require.Equal(t, "2026-11-01T00:00:00Z", repo.seenFilters.EndTime.Format(time.RFC3339))
}

func TestUsageHandlerInternalStatsDefaultsToLast30Days(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &usageStatsFakeRepo{}
	handler := newInternalUsageStatsHandler(repo)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/internal/usage/stats", nil)
	handler.InternalStats(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "none", repo.seenGroupBy)
	require.NotNil(t, repo.seenFilters.StartTime)
	require.NotNil(t, repo.seenFilters.EndTime)
	require.InDelta(t, (30 * 24 * time.Hour).Seconds(), repo.seenFilters.EndTime.Sub(*repo.seenFilters.StartTime).Seconds(), 5)
}

func TestUsageHandlerInternalStatsRejectsInvalidParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newInternalUsageStatsHandler(&usageStatsFakeRepo{})

	for _, target := range []string{
		"/api/v1/internal/usage/stats?group_by=unknown",
		"/api/v1/internal/usage/stats?limit=0",
		"/api/v1/internal/usage/stats?order=random",
		"/api/v1/internal/usage/stats?start_time=bad",
		"/api/v1/internal/usage/stats?start_time=2026-10-20&end_time=2026-10-01",
		"/api/v1/internal/usage/stats?start_time=2020-01-01&end_time=2026-10-01",
		"/api/v1/internal/usage/stats?user_id=0",
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, target, nil)
		handler.InternalStats(c)
		require.Equal(t, http.StatusBadRequest, recorder.Code, target)
	}
}
