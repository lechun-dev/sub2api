package repository

import (
	"context"
	"encoding/json"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestClientAccessCAS(t *testing.T) {
	for _, rows := range []int64{0, 1} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
		repo := &settingRepository{client: client}
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "settings" SET`).WithArgs("new", sqlmock.AnyArg(), service.SettingKeyClientAccess, "old").WillReturnResult(sqlmock.NewResult(0, rows))
		if rows == 0 {
			mock.ExpectRollback()
		} else {
			mock.ExpectCommit()
		}
		err = repo.CompareAndSetClientAccess(context.Background(), "old", "new", nil)
		if rows == 0 {
			require.ErrorIs(t, err, service.ErrClientAccessConflict)
		} else {
			require.NoError(t, err)
		}
		mock.ExpectClose()
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestClientAccessGroupCopyAndDelete(t *testing.T) {
	cfg := service.DefaultClientAccessConfig()
	cfg.Groups["38"] = service.GroupClientPolicy{Mode: "deny", ClientIDs: []string{"codex"}}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	copied, changed, err := clientAccessGroupValue(string(raw), 38, 99)
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, json.Unmarshal([]byte(copied), &cfg))
	require.Equal(t, cfg.Groups["38"], cfg.Groups["99"])
	deleted, changed, err := clientAccessGroupValue(copied, 38, 0)
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, json.Unmarshal([]byte(deleted), &cfg))
	// 2026-10-13 coder(lq): 回读到新结构体，防止 JSON 解码沿用旧 map 条目造成删除断言失真。
	var after service.ClientAccessConfig
	require.NoError(t, json.Unmarshal([]byte(deleted), &after))
	require.NotContains(t, after.Groups, "38")
	require.Contains(t, after.Groups, "99")
	_, changed, err = clientAccessGroupValue(deleted, 38, 0)
	require.NoError(t, err)
	require.False(t, changed)
	_, _, err = clientAccessGroupValue("broken", 38, 99)
	require.Error(t, err)
}
