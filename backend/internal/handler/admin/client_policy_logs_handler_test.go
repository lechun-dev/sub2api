package admin

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type clientPolicyLogRepo struct {
	service.OpsRepository
	filter service.ClientPolicyDenialFilter
}

func (r *clientPolicyLogRepo) ListClientPolicyDenials(_ context.Context, filter service.ClientPolicyDenialFilter) (*service.ClientPolicyDenialList, error) {
	r.filter = filter
	return &service.ClientPolicyDenialList{Items: []*service.ClientPolicyDenialLog{{ID: 1, GroupID: filter.GroupID}}, Total: 1, Page: filter.Page, PageSize: filter.PageSize}, nil
}
func TestClientPolicyDenialHandlerValidationAndGroupScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &clientPolicyLogRepo{}
	ops := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(ops)
	request := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/admin/ops/client-policy-denials?"+query, nil)
		handler.ListClientPolicyDenials(c)
		return w
	}
	for _, query := range []string{"", "group_id=0", "group_id=-1", "group_id=no", "group_id=38&start_time=no", "group_id=38&start_time=2026-01-01T00:00:00Z&end_time=2026-03-01T00:00:00Z", "group_id=38&start_time=2026-01-01T00:00:00Z&end_time=2026-01-01T00:00:00Z", "group_id=38&page=100001"} {
		require.Equal(t, 400, request(query).Code, query)
	}
	w := request("group_id=38&page=2&page_size=20&start_time=2026-10-13T00:00:00Z&end_time=2026-10-14T00:00:00Z")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"group_id":38`)
	require.Equal(t, int64(38), repo.filter.GroupID)
	require.Equal(t, 2, repo.filter.Page)
	require.Equal(t, 24*time.Hour, repo.filter.EndTime.Sub(repo.filter.StartTime))
	ops.SetMonitoringEnabled(false)
	require.Equal(t, 404, request("group_id=38").Code)
}
