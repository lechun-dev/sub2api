package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestClientPolicyUserAgentSanitization(t *testing.T) {
	t.Parallel()
	for _, ua := range []string{"deepseek-harness/1.2.3", "codex_cli_rs/1.2.3", "curl/8.9", "Mozilla/5.0", ""} {
		require.Equal(t, ua, SanitizeClientPolicyUserAgent(ua))
	}
	raw := "deepseek-harness/1 (+https://name:pass@example.test/private?token=url-secret) Bearer bearer-secret sk-private-test api_key=key-secret access_token=access-secret cookie=cookie-secret opaque-real-key\r\n\x00\u202e"
	got := SanitizeClientPolicyUserAgent(raw, "opaque-real-key")
	for _, secret := range []string{"name:pass", "url-secret", "bearer-secret", "sk-private-test", "key-secret", "access-secret", "cookie-secret", "opaque-real-key", "\r", "\n", "\x00", "\u202e"} {
		require.NotContains(t, got, secret)
	}
	require.Contains(t, got, "deepseek-harness/1")
	require.Equal(t, got, SanitizeClientPolicyUserAgent(got))
	for _, raw := range []string{strings.Repeat("界", 10000), strings.Repeat("a", 10000), "curl/8\xff"} {
		got := SanitizeClientPolicyUserAgent(raw)
		require.LessOrEqual(t, len(got), 512)
		require.True(t, utf8.ValidString(got))
	}
	require.NotContains(t, SanitizeClientPolicyUserAgent(`{"api_key":"hidden-json"}`), "hidden-json")
}

func TestClientPolicyRecognitionReasonsDoNotRelaxAccess(t *testing.T) {
	cfg := DefaultClientAccessConfig()
	cfg.Groups["38"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"deepseek_harness"}}
	for _, test := range []struct{ ua, reason string }{{"", "user_agent_missing"}, {"curl/8", "no_matching_rule"}, {"codex_cli_rs/1", "matched_registered_client"}} {
		headers := http.Header{}
		headers.Set("User-Agent", test.ua)
		decision := EvaluateClientAccess(&cfg, 38, headers)
		require.False(t, decision.Allowed)
		require.Equal(t, test.reason, decision.Denial.RecognitionReason)
		require.NotNil(t, decision.Denial.UserAgentPresent)
		require.Equal(t, test.ua != "", *decision.Denial.UserAgentPresent)
		encoded, err := json.Marshal(decision.Denial)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "curl/8", "raw UA belongs only in the bounded column, not the policy JSON")
	}
	headers := http.Header{}
	headers.Set("User-Agent", "deepseek-harness/100.2.3")
	require.True(t, EvaluateClientAccess(&cfg, 38, headers).Allowed)
	cfg.Clients = append(cfg.Clients, ClientDefinition{ID: "custom", Name: "Custom", Enabled: true, Detector: "headers", Match: "all", Rules: []ClientHeaderRule{{Header: "X-Client", Operator: "exact", Value: "custom"}}})
	headers.Del("User-Agent")
	headers.Set("X-Client", "custom")
	require.Equal(t, "matched_registered_client", EvaluateClientAccess(&cfg, 38, headers).Denial.RecognitionReason)
	cfg.Groups["38"] = GroupClientPolicy{Mode: "deny", ClientIDs: []string{"deepseek_harness"}}
	headers.Set("User-Agent", "deepseek-harness/1")
	decision := EvaluateClientAccess(&cfg, 38, headers)
	require.Equal(t, "client_forbidden", decision.Code)
	require.Equal(t, "matched_registered_client", decision.Denial.RecognitionReason)
}
