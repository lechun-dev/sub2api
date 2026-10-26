package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	// InternalIdentityEmailHeader 内部系统声明的用户邮箱。
	InternalIdentityEmailHeader = "X-Lechun-Email"
	// InternalIdentityTimestampHeader Unix 秒时间戳，用于限制重放窗口。
	InternalIdentityTimestampHeader = "X-Lechun-Timestamp"
	// InternalIdentitySignatureHeader 十六进制 HMAC-SHA256 签名。
	InternalIdentitySignatureHeader = "X-Lechun-Signature"

	internalIdentityAuthMethod = "internal_signed"
	internalIdentityMaxSkew    = 300
)

// InternalIdentityUserLookup 内部签名身份校验所需的最小用户查询能力。
type InternalIdentityUserLookup interface {
	GetByEmail(ctx context.Context, email string) (*service.User, error)
}

// InternalIdentityAuth 校验内部系统的签名身份断言：请求声称代表某个邮箱，用共享密钥签名。
//
// 签名串固定为：email + "\n" + timestamp + "\n" + method + "\n" + path。
// 通过后按 sub2api 用户角色判断是否管理员；非管理员直接 403。共享密钥只用于身份断言，
// 无法调用其他管理接口（内部路由组只挂载只读查询）。
func InternalIdentityAuth(users InternalIdentityUserLookup, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if users == nil || cfg == nil || !cfg.InternalIdentity.Enabled || strings.TrimSpace(cfg.InternalIdentity.Secret) == "" {
			// 未启用时按不存在处理，避免暴露内部接口。
			AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Not found")
			return
		}

		email := strings.TrimSpace(c.GetHeader(InternalIdentityEmailHeader))
		timestamp := strings.TrimSpace(c.GetHeader(InternalIdentityTimestampHeader))
		signature := strings.TrimSpace(c.GetHeader(InternalIdentitySignatureHeader))
		if email == "" || timestamp == "" || signature == "" {
			AbortWithError(c, http.StatusUnauthorized, "INVALID_INTERNAL_SIGNATURE", "Invalid internal identity signature")
			return
		}

		unix, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, "INVALID_INTERNAL_SIGNATURE", "Invalid internal identity signature")
			return
		}
		skew := cfg.InternalIdentity.MaxSkewSeconds
		if skew <= 0 {
			skew = internalIdentityMaxSkew
		}
		now := time.Now().Unix()
		if unix < now-int64(skew) || unix > now+int64(skew) {
			AbortWithError(c, http.StatusUnauthorized, "INTERNAL_SIGNATURE_EXPIRED", "Internal identity signature expired")
			return
		}

		canonical := email + "\n" + timestamp + "\n" + c.Request.Method + "\n" + c.Request.URL.Path
		mac := hmac.New(sha256.New, []byte(cfg.InternalIdentity.Secret))
		_, _ = mac.Write([]byte(canonical))
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(expected), []byte(strings.ToLower(signature))) {
			AbortWithError(c, http.StatusUnauthorized, "INVALID_INTERNAL_SIGNATURE", "Invalid internal identity signature")
			return
		}

		user, err := users.GetByEmail(c.Request.Context(), email)
		if err != nil || user == nil || !user.IsActive() {
			AbortWithError(c, http.StatusUnauthorized, "INVALID_INTERNAL_IDENTITY", "Internal identity is not an active user")
			return
		}
		if !user.IsAdmin() {
			AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "仅 sub2api 管理员可使用该接口")
			return
		}

		c.Set(string(ContextKeyUser), AuthSubject{UserID: user.ID, Concurrency: user.Concurrency})
		c.Set(string(ContextKeyUserRole), user.Role)
		c.Set(ContextKeyAuthEmail, user.Email)
		c.Set("auth_method", internalIdentityAuthMethod)
		c.Next()
	}
}
