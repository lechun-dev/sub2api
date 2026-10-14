package service

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

var clientPolicyUASecret = regexp.MustCompile(`(?i)\b(?:bearer\s+[^\s;,)]+|sk-[a-z0-9_-]+)`)
var clientPolicyUAURL = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s()<>]+`)

// 2026-10-14 coder(lq): UA 仅作管理员排错依据；写队列前过滤控制字符、凭据和 URL，限制 512 字节，不改变用于鉴权的原始头。
func SanitizeClientPolicyUserAgent(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	value = strings.ToValidUTF8(value, "")
	if len(value) > 8192 {
		value = value[:8192]
		value = strings.ToValidUTF8(value, "")
	}
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, value)
	value = clientPolicyUAURL.ReplaceAllString(value, "[URL omitted]")
	value = clientPolicyUASecret.ReplaceAllString(value, "[REDACTED]")
	value = logredact.RedactText(value, "authorization", "api_key", "api-key", "x-api-key", "token", "secret", "cookie", "session_id")
	value = strings.TrimSpace(value)
	if len(value) > 512 {
		value = value[:509]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
		value += "…"
	}
	return value
}
