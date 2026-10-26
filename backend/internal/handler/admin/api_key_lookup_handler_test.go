package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type apiKeyLookupFakeAdminService struct {
	service.AdminService
	user       *service.User
	keys       []service.APIKey
	err        error
	seenEmail  string
	seenStatus string
}

func (f *apiKeyLookupFakeAdminService) FindAPIKeysByEmail(_ context.Context, email, status string) (*service.User, []service.APIKey, error) {
	f.seenEmail = email
	f.seenStatus = status
	return f.user, f.keys, f.err
}

func TestAdminAPIKeyIssuanceHandlerGetByEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(2)
	fake := &apiKeyLookupFakeAdminService{
		user: &service.User{ID: 7, Username: "张三", Email: "zhang@example.com", Status: "active"},
		keys: []service.APIKey{{
			ID:        11,
			UserID:    7,
			Key:       "sk-test-key",
			Name:      "默认",
			Status:    "active",
			GroupID:   &groupID,
			Group:     &service.Group{ID: groupID, Name: "Claude 分组"},
			Quota:     10,
			QuotaUsed: 1.5,
		}},
	}
	handler := NewAdminAPIKeyIssuanceHandler(fake, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-keys/by-email?email=zhang@example.com&status=active", nil)
	handler.GetByEmail(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "zhang@example.com", fake.seenEmail)
	require.Equal(t, "active", fake.seenStatus)

	var body struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
				Email    string `json:"email"`
			} `json:"user"`
			APIKeys []struct {
				ID        int64  `json:"id"`
				Key       string `json:"key"`
				GroupName string `json:"group_name"`
			} `json:"api_keys"`
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, int64(7), body.Data.User.ID)
	require.Equal(t, "张三", body.Data.User.Username)
	require.Len(t, body.Data.APIKeys, 1)
	require.Equal(t, "sk-test-key", body.Data.APIKeys[0].Key)
	require.Equal(t, "Claude 分组", body.Data.APIKeys[0].GroupName)
	require.Equal(t, 1, body.Data.Total)
}

func TestAdminAPIKeyIssuanceHandlerGetByEmailRequiresEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAdminAPIKeyIssuanceHandler(&apiKeyLookupFakeAdminService{}, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-keys/by-email", nil)
	handler.GetByEmail(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAdminAPIKeyIssuanceHandlerGetByEmailNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &apiKeyLookupFakeAdminService{err: service.ErrUserNotFound}
	handler := NewAdminAPIKeyIssuanceHandler(fake, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-keys/by-email?email=missing@example.com", nil)
	handler.GetByEmail(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}
