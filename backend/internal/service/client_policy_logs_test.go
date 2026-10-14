package service

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClientPolicyDenialSnapshotCodesAndBounds(t *testing.T) {
	cfg := DefaultClientAccessConfig()
	now := time.Now().UTC()
	cfg.Groups["38"] = GroupClientPolicy{Mode: "allow", ClientIDs: []string{"codex"}, UpdatedAt: &now}
	for _, test := range []struct{ ua, code string }{{"curl/8", "client_unrecognized"}, {"deepseek-harness/1", "client_not_allowed"}, {"codex_cli_rs/1", ""}} {
		headers := http.Header{}
		headers.Set("User-Agent", test.ua)
		decision := EvaluateClientAccess(&cfg, 38, headers)
		if test.code == "" {
			require.True(t, decision.Allowed)
			require.Nil(t, decision.Denial)
			continue
		}
		require.Equal(t, test.code, decision.Denial.Code)
		require.Equal(t, now, *decision.Denial.PolicyUpdatedAt)
		require.Equal(t, "codex", decision.Denial.SelectedClients[0].ID)
		if test.code == "client_unrecognized" {
			require.Empty(t, decision.Denial.DetectedClients)
		} else {
			require.Equal(t, "deepseek_harness", decision.Denial.DetectedClients[0].ID)
		}
	}
	clients := make([]ClientPolicyClient, 100)
	for i := range clients {
		clients[i] = ClientPolicyClient{ID: strings.Repeat("x", 64), Name: strings.Repeat("<", 100)}
	}
	snapshot := ClientPolicySnapshot{Code: "client_forbidden", Mode: "deny", DetectedClients: boundedPolicyClients(clients), SelectedClients: boundedPolicyClients(clients), DetectedCount: 100, SelectedCount: 100}
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.Less(t, len(encoded), 8192)
	require.Len(t, snapshot.DetectedClients, 8)
	var decoded ClientPolicySnapshot
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, 100, decoded.DetectedCount)
}
