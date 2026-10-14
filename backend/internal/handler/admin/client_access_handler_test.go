package admin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type clientAccessHandlerRepo struct {
	service.SettingRepository
	raw      string
	writeErr error
	writes   int
}

func (r *clientAccessHandlerRepo) GetValue(context.Context, string) (string, error) {
	if r.raw == "" {
		return "", service.ErrSettingNotFound
	}
	return r.raw, nil
}
func (r *clientAccessHandlerRepo) CompareAndSetClientAccess(_ context.Context, old, next string, _ []int64) error {
	if r.writeErr != nil {
		return r.writeErr
	}
	if r.raw != old {
		return service.ErrClientAccessConflict
	}
	r.raw, r.writes = next, r.writes+1
	return nil
}

func TestClientAccessHandlerValidationAndConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name      string
		mutate    func(*service.ClientAccessConfig)
		writerErr error
		status    int
	}{
		{"valid", func(*service.ClientAccessConfig) {}, nil, 200},
		{"empty allow", func(c *service.ClientAccessConfig) { c.Groups["38"] = service.GroupClientPolicy{Mode: "allow"} }, nil, 400},
		{"revision conflict", func(c *service.ClientAccessConfig) { c.Revision = "stale" }, nil, 409},
		{"CAS conflict", func(*service.ClientAccessConfig) {}, service.ErrClientAccessConflict, 409},
		{"deleted group", func(c *service.ClientAccessConfig) {
			c.Groups["38"] = service.GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex"}}
		}, service.ErrGroupNotFound, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &clientAccessHandlerRepo{writeErr: tc.writerErr}
			h := &SettingHandler{settingService: service.NewSettingService(repo, nil)}
			router := gin.New()
			router.PUT("/client-access", h.SaveClientAccess)
			cfg := service.DefaultClientAccessConfig()
			tc.mutate(&cfg)
			raw, err := json.Marshal(cfg)
			require.NoError(t, err)
			req := httptest.NewRequest("PUT", "/client-access", strings.NewReader(string(raw)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status != 200 {
				require.Zero(t, repo.writes)
			}
		})
	}
}

func TestClientAccessHandlerCorruptConfig(t *testing.T) {
	h := &SettingHandler{settingService: service.NewSettingService(&clientAccessHandlerRepo{raw: "broken"}, nil)}
	router := gin.New()
	router.GET("/client-access", h.GetClientAccess)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/client-access", nil))
	require.Equal(t, 503, w.Code)
	require.NotContains(t, w.Body.String(), "broken")
}
