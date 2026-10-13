package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 2026-10-13 coder(lq): CAS 在数据库中执行，跨实例保存也不会覆盖其他管理员的配置；不新增表。
func (r *settingRepository) CompareAndSetClientAccess(ctx context.Context, expected, value string, groupIDs []int64) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	if err := lockLiveGroups(ctx, client, groupIDs); err != nil {
		return err
	}
	if expected == "" {
		if err := client.Setting.Create().SetKey(service.SettingKeyClientAccess).SetValue("").OnConflictColumns(setting.FieldKey).DoNothing().Exec(ctx); err != nil {
			return err
		}
	}
	count, err := client.Setting.Update().Where(setting.KeyEQ(service.SettingKeyClientAccess), setting.ValueEQ(expected)).SetValue(value).SetUpdatedAt(time.Now()).Save(ctx)
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrClientAccessConflict
	}
	return tx.Commit()
}

// 2026-10-13 coder(lq): 分组复制/删除与访问策略同事务提交，避免复制得到不受限制的新分组。
func mutateClientAccessGroup(ctx context.Context, client *ent.Client, sourceID, targetID int64) error {
	var raw string
	err := scanSingleRow(ctx, client, "SELECT value FROM settings WHERE key = $1 FOR UPDATE", []any{service.SettingKeyClientAccess}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	value, changed, err := clientAccessGroupValue(raw, sourceID, targetID)
	if err != nil || !changed {
		return err
	}
	return client.Setting.Update().Where(setting.KeyEQ(service.SettingKeyClientAccess)).SetValue(value).SetUpdatedAt(time.Now()).Exec(ctx)
}

func clientAccessGroupValue(raw string, sourceID, targetID int64) (string, bool, error) {
	var cfg service.ClientAccessConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return "", false, err
	}
	if err := service.ValidateClientAccessConfig(&cfg); err != nil {
		return "", false, err
	}
	key := strconv.FormatInt(sourceID, 10)
	policy, exists := cfg.Groups[key]
	if !exists {
		return raw, false, nil
	}
	if targetID == 0 {
		delete(cfg.Groups, key)
	} else {
		cfg.Groups[strconv.FormatInt(targetID, 10)] = policy
	}
	cfg.Revision = ""
	if err := service.ValidateClientAccessConfig(&cfg); err != nil {
		return "", false, err
	}
	value, err := json.Marshal(cfg)
	return string(value), true, err
}
