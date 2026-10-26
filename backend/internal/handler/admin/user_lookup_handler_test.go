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

type userByEmailFakeAdminService struct {
	service.AdminService
	user      *service.User
	err       error
	seenEmail string
}

func (f *userByEmailFakeAdminService) FindUserByEmail(_ context.Context, email string) (*service.User, error) {
	f.seenEmail = email
	return f.user, f.err
}

func TestUserHandlerGetByEmailReturnsAdminFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &userByEmailFakeAdminService{user: &service.User{
		ID: 7, Username: "张三", Email: "zhang@example.com", Role: service.RoleAdmin, Status: service.StatusActive,
	}}
	handler := NewUserHandler(fake, nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/by-email?email=zhang@example.com", nil)
	handler.GetByEmail(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "zhang@example.com", fake.seenEmail)

	var body struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				ID    int64  `json:"id"`
				Email string `json:"email"`
				Role  string `json:"role"`
			} `json:"user"`
			IsAdmin bool `json:"is_admin"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, int64(7), body.Data.User.ID)
	require.Equal(t, service.RoleAdmin, body.Data.User.Role)
	require.True(t, body.Data.IsAdmin)
}

func TestUserHandlerGetByEmailNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &userByEmailFakeAdminService{user: &service.User{
		ID: 8, Username: "李四", Email: "li@example.com", Role: "user", Status: service.StatusActive,
	}}
	handler := NewUserHandler(fake, nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/by-email?email=li@example.com", nil)
	handler.GetByEmail(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data struct {
			IsAdmin bool `json:"is_admin"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Data.IsAdmin)
}

func TestUserHandlerGetByEmailValidationAndNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewUserHandler(&userByEmailFakeAdminService{}, nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/by-email", nil)
	handler.GetByEmail(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	fake := &userByEmailFakeAdminService{err: service.ErrUserNotFound}
	handler = NewUserHandler(fake, nil, nil, nil, nil, nil, nil)
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/by-email?email=missing@example.com", nil)
	handler.GetByEmail(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}
