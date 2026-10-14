package repository

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestClientPolicyDenialLogsQueryScopeAndCorruptIsolation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	start := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filter := service.ClientPolicyDenialFilter{GroupID: 38, StartTime: start, EndTime: end, Page: 2, PageSize: 20}
	mock.ExpectQuery(`SELECT COUNT\(\*\).*WHERE l.error_type = \$1 AND l.group_id = \$2 AND l.created_at >= \$3 AND l.created_at < \$4`).WithArgs(service.ClientPolicyDenialType, int64(38), start, end).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(22))
	rows := sqlmock.NewRows([]string{"id", "created_at", "group_id", "user_id", "api_key_id", "client_ip", "request_id", "request_path", "error_body", "username"}).AddRow(2, end, 38, nil, nil, "", "req-2", "/responses", `{"code":"client_forbidden","mode":"deny","detected_clients":[],"detected_count":0,"selected_count":1,"selected_clients":[{"id":"codex","name":"Old Codex"}],"secret":"not-returned"}`, "").AddRow(1, start, 38, 72, 41, "192.0.2.1", "req-1", "/responses", "broken-json-secret", "  张三  ")
	mock.ExpectQuery(`SELECT l.id,l.created_at,l.group_id.*LEFT JOIN users u ON u.id = l.user_id AND u.deleted_at IS NULL.*ORDER BY l.created_at DESC,l.id DESC LIMIT \$5 OFFSET \$6`).WithArgs(service.ClientPolicyDenialType, int64(38), start, end, 20, 20).WillReturnRows(rows)
	result, err := (&opsRepository{db: db}).ListClientPolicyDenials(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, 22, result.Total)
	require.Len(t, result.Items, 2)
	require.Nil(t, result.Items[0].UserID)
	require.Empty(t, result.Items[0].Username)
	require.Equal(t, "张三", result.Items[1].Username)
	require.Equal(t, "Old Codex", result.Items[0].Snapshot.SelectedClients[0].Name)
	require.Nil(t, result.Items[1].Snapshot)
	require.Equal(t, int64(41), *result.Items[1].APIKeyID)
	body, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(body), "not-returned")
	require.NotContains(t, string(body), "broken-json-secret")
	require.NoError(t, mock.ExpectationsWereMet())
	for _, invalid := range []service.ClientPolicyDenialFilter{{GroupID: 0}, {GroupID: 38, StartTime: start, EndTime: end, Page: 100001}, {GroupID: 38, StartTime: start, EndTime: start.Add(31 * 24 * time.Hour)}} {
		_, err := (&opsRepository{db: db}).ListClientPolicyDenials(context.Background(), invalid)
		require.Error(t, err)
	}
}

func TestClientPolicyDenialLogsRejectIncompleteSnapshot(t *testing.T) {
	for _, raw := range []string{`{"code":"client_forbidden"}`, `{"code":"client_forbidden","mode":"allow","detected_clients":[],"selected_clients":[]}`, `{"code":"client_unrecognized","mode":"allow","selected_count":1,"selected_clients":[{"id":"codex","name":"Codex"}],"detected_clients":null}`} {
		var snapshot service.ClientPolicySnapshot
		require.NoError(t, json.Unmarshal([]byte(raw), &snapshot))
		require.False(t, validClientPolicySnapshot(snapshot))
	}
}
